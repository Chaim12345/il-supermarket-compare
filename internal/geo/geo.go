// Package geo geocodes Israeli addresses with OpenStreetMap Nominatim and
// caches every lookup permanently in SQLite. Nominatim allows one request per
// second, so the cache is what makes repeat queries fast.
package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"il-supermarket-web/internal/store"
)

const (
	nominatimURL = "https://nominatim.openstreetmap.org/search"
	photonURL    = "https://photon.komoot.io/api"
	geoUA        = "il-supermarket-compare/1.1 (personal price comparison)"
)

type Geocoder struct {
	db   *store.DB
	mu   sync.Mutex
	last time.Time
	http *http.Client
}

func New(db *store.DB) *Geocoder {
	return &Geocoder{db: db, http: &http.Client{Timeout: 30 * time.Second}}
}

// Lookup returns coordinates for a free-text query. found=false is a valid,
// cached answer meaning "Nominatim has no such place". Transient failures (rate
// limiting returns HTML, not JSON) are retried with backoff; the caller should
// treat a returned error as "unknown, try again later" rather than "no such
// place", so it is not cached.
func (g *Geocoder) Lookup(ctx context.Context, query string) (lat, lon float64, found bool, err error) {
	if la, lo, f, ok := g.db.GeocodeGet(query); ok {
		return la, lo, f, nil
	}

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 3 * time.Second):
			case <-ctx.Done():
				return 0, 0, false, ctx.Err()
			}
		}
		lat, lon, found, retry, err := g.query(ctx, query)
		if err == nil {
			return lat, lon, found, nil
		}
		lastErr = err
		if !retry {
			return 0, 0, false, err
		}
	}
	return 0, 0, false, lastErr
}

func (g *Geocoder) query(ctx context.Context, query string) (lat, lon float64, found, retry bool, err error) {
	// Nominatim is precise but blocks bursts; Photon is the fallback. Try each
	// in turn and accept the first provider that answers.
	nq := nominatimURL + "?" + url.Values{
		"q": {query}, "format": {"json"}, "limit": {"1"}, "countrycodes": {"il"},
	}.Encode()
	lat, lon, found, nErr := g.fetch(ctx, query, nq, parseNominatim)
	if nErr == nil && found {
		return lat, lon, true, false, nil
	}

	pq := photonURL + "?" + url.Values{"q": {query}, "limit": {"1"}}.Encode()
	if plat, plon, pfound, pErr := g.fetch(ctx, query, pq, parsePhoton); pErr == nil {
		if pfound {
			return plat, plon, true, false, nil
		}
		// Both providers answered and neither knows the place: cache the miss.
		_ = g.db.GeocodePut(query, 0, 0, false)
		return 0, 0, false, false, nil
	}

	if nErr != nil {
		return 0, 0, false, true, nErr
	}
	return 0, 0, false, false, nil
}

// fetch performs one rate-limited request and parses the body. A non-JSON body
// (rate limiting returns HTML) is reported as retryable. cacheKey is the query
// the result is stored under.
func (g *Geocoder) fetch(ctx context.Context, cacheKey, rawURL string, parse func([]byte) (float64, float64, bool, error)) (lat, lon float64, found bool, err error) {
	g.mu.Lock()
	if wait := 1200*time.Millisecond - time.Since(g.last); wait > 0 {
		time.Sleep(wait)
	}
	g.last = time.Now()
	g.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, 0, false, err
	}
	req.Header.Set("User-Agent", geoUA)
	resp, err := g.http.Do(req)
	if err != nil {
		return 0, 0, false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, 0, false, err
	}
	if resp.StatusCode != http.StatusOK || len(body) == 0 || body[0] == '<' {
		return 0, 0, false, fmt.Errorf("%s: status %d", rawURL, resp.StatusCode)
	}
	lat, lon, found, err = parse(body)
	if err != nil {
		return 0, 0, false, err
	}
	if found {
		if err := g.db.GeocodePut(cacheKey, lat, lon, true); err != nil {
			return 0, 0, false, err
		}
	}
	return lat, lon, found, nil
}

func parseNominatim(body []byte) (lat, lon float64, found bool, err error) {
	var hits []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.Unmarshal(body, &hits); err != nil {
		return 0, 0, false, err
	}
	if len(hits) == 0 {
		return 0, 0, false, nil
	}
	if _, err := fmt.Sscanf(hits[0].Lat, "%f", &lat); err != nil {
		return 0, 0, false, err
	}
	if _, err := fmt.Sscanf(hits[0].Lon, "%f", &lon); err != nil {
		return 0, 0, false, err
	}
	return lat, lon, true, nil
}

func parsePhoton(body []byte) (lat, lon float64, found bool, err error) {
	var res struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				CountryCode string `json:"countrycode"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return 0, 0, false, err
	}
	for _, f := range res.Features {
		if len(f.Geometry.Coordinates) != 2 {
			continue
		}
		if cc := f.Properties.CountryCode; cc != "" && cc != "IL" {
			continue
		}
		// GeoJSON is [lon, lat].
		return f.Geometry.Coordinates[1], f.Geometry.Coordinates[0], true, nil
	}
	return 0, 0, false, nil
}

var (
	parenRe  = regexp.MustCompile(`\(.*?\)`)
	streetRe = regexp.MustCompile(`\b(רח(וב)?['.]?|שד(רות)?['.]?)\s*`)
	abbrRe   = regexp.MustCompile(`ר"ג|ת"א|ב"ב|פינת.*|מ\.מסחרי|מרכז מסחרי`)
	spaceRe  = regexp.MustCompile(`\s+`)
	numRe    = regexp.MustCompile(`\d+[-\d]*`)
)

func cleanAddress(addr, city string) string {
	a := parenRe.ReplaceAllString(addr, " ")
	a = streetRe.ReplaceAllString(a, "")
	a = abbrRe.ReplaceAllString(a, " ")
	a = strings.ReplaceAll(a, city, " ")
	a = strings.NewReplacer(`"`, "", "״", "").Replace(a)
	a = spaceRe.ReplaceAllString(a, " ")
	return strings.Trim(a, " ,-")
}

// LocateStore tries progressively looser queries for a branch address.
func (g *Geocoder) LocateStore(ctx context.Context, address, city string) (lat, lon float64, ok bool, err error) {
	if address == "" || strings.HasPrefix(address, "http") {
		return 0, 0, false, nil
	}
	clean := cleanAddress(address, city)
	tries := []string{fmt.Sprintf("%s, %s", address, city), fmt.Sprintf("%s, %s", clean, city)}
	if street := strings.TrimSpace(numRe.ReplaceAllString(clean, "")); street != "" && street != clean {
		tries = append(tries, fmt.Sprintf("%s, %s", street, city))
	}
	seen := map[string]bool{}
	var lastErr error
	for _, q := range tries {
		if q == "" || seen[q] {
			continue
		}
		seen[q] = true
		la, lo, f, err := g.Lookup(ctx, q)
		if err != nil {
			// A transient failure: remember it but keep trying looser queries,
			// and let the caller decide. Do not cache a negative result.
			lastErr = err
			continue
		}
		if f {
			return la, lo, true, nil
		}
	}
	return 0, 0, false, lastErr
}

// BoundingBox returns a lat/lon box around a point, padded for the radius.
func BoundingBox(lat, lon, radiusKm float64) (minLat, maxLat, minLon, maxLon float64) {
	dLat := radiusKm / 111.0
	dLon := radiusKm / (111.0 * math.Cos(lat*math.Pi/180))
	return lat - dLat, lat + dLat, lon - dLon, lon + dLon
}

// HaversineKm is the great-circle distance in kilometres.
func HaversineKm(aLat, aLon, bLat, bLon float64) float64 {
	const R = 6371
	p1, p2 := aLat*math.Pi/180, bLat*math.Pi/180
	dp := p2 - p1
	dl := (bLon - aLon) * math.Pi / 180
	h := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * R * math.Asin(math.Sqrt(h))
}
