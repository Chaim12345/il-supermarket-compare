// Package ingest refreshes store locations and coordinates. It is used both by
// the `ingest` command and by the server's daily self-refresh.
package ingest

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"il-supermarket-web/internal/catalog"
	"il-supermarket-web/internal/geo"
	"il-supermarket-web/internal/portals"
	"il-supermarket-web/internal/store"
)

type Options struct {
	// Limit caps how many stores are geocoded (0 = all). Stores beyond the cap
	// are still stored, without coordinates.
	Limit int
	// Chains restricts the run to specific chain keys (empty = all).
	Chains []string
	// SnapshotDir, when set, also writes a static stores.json/meta.json.
	SnapshotDir string
}

type Stats struct {
	Stored    int
	Geocoded  int
	Chains    int
	TotalInDB int
}

func Run(ctx context.Context, db *store.DB, geocoder *geo.Geocoder, opt Options) (Stats, error) {
	var st Stats

	codes, err := portals.FetchCityCodes(ctx)
	if err != nil {
		log.Printf("ingest: city codes unavailable (%v); cities may show as numbers", err)
	}

	wanted := map[string]bool{}
	for _, k := range opt.Chains {
		wanted[strings.TrimSpace(k)] = true
	}

	var rows []store.StoreRow
	for _, c := range portals.Chains {
		if len(wanted) > 0 && !wanted[c.Key] {
			continue
		}
		stores, err := portals.FetchStores(ctx, c)
		if err != nil {
			log.Printf("ingest: skip %s: %v", c.Key, err)
			continue
		}
		st.Chains++
		log.Printf("ingest: %s -> %d stores", c.Key, len(stores))

		var chainRows []store.StoreRow
		for _, s := range stores {
			city := s.City
			if isDigits(city) {
				if name, ok := codes[strings.TrimLeft(city, "0")]; ok {
					city = name
				}
			}
			// Some chains (e.g. yohananof) leave City empty or "0" and put the
			// locality in the store name; recover it so the branch can be placed.
			if city == "" || isDigits(city) {
				city = localityFromName(s.Name)
			}
			if city == "" {
				continue // no usable city, cannot place it
			}
			row := store.StoreRow{
				ID: c.Key + ":" + s.ID, Chain: c.Key, Name: s.Name,
				Address: s.Address, City: city, SubChain: s.SubChain, Kind: string(c.Kind),
			}
			if opt.Limit == 0 || st.Geocoded < opt.Limit {
				lat, lon, ok, err := geocoder.LocateStore(ctx, s.Address, city)
				if err == nil && ok {
					row.Lat, row.Lon, row.Approx = lat, lon, false
				} else {
					if err != nil {
						log.Printf("ingest: geocode %s: %v", row.ID, err)
					}
					// No precise hit (or a transient failure): fall back to the
					// city centre and flag it approximate.
					if la, lo, found, _ := geocoder.Lookup(ctx, city+", ישראל"); found {
						row.Lat, row.Lon, row.Approx = la, lo, true
					}
				}
				st.Geocoded++
			}
			if row.Lat != 0 || row.Lon != 0 {
				chainRows = append(chainRows, row)
			}
		}
		// Flush per chain so a crash or timeout keeps the finished work.
		if err := db.UpsertStores(chainRows); err != nil {
			return st, err
		}
		rows = append(rows, chainRows...)
		st.Stored = len(rows)
		log.Printf("ingest: %s stored (%d with coordinates so far)", c.Key, st.Stored)
	}

	_ = db.MetaSet("stores_refreshed", time.Now().UTC().Format(time.RFC3339))
	st.TotalInDB, _ = db.StoreCount()
	log.Printf("ingest: stored %d rows, %d in db", st.Stored, st.TotalInDB)

	if opt.SnapshotDir != "" {
		if err := WriteSnapshot(opt.SnapshotDir, rows); err != nil {
			return st, err
		}
	}
	return st, nil
}

// WriteSnapshot emits the static files used by the optional GitHub Pages mode.
func WriteSnapshot(dir string, rows []store.StoreRow) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeJSONFile(dir+"/stores.json", rows); err != nil {
		return err
	}
	return writeJSONFile(dir+"/meta.json", map[string]any{
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
		"catalog":     catalog.Items,
		"chains":      portals.Chains,
		"storeCount":  len(rows),
	})
}

func writeJSONFile(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(v)
}

// localityFromName recovers a locality from a store name when the chain does not
// provide a city. It strips the banner prefix and common branch suffixes, leaving
// the place name ("יוחננוף מפוח" -> "מפוח", "יוחננוף אור יהודה" -> "אור יהודה").
func localityFromName(name string) string {
	name = strings.TrimSpace(storeNameStripRe.ReplaceAllString(name, " "))
	name = strings.TrimSpace(branchSuffixRe.ReplaceAllString(name, " "))
	return strings.TrimSpace(strings.Join(strings.Fields(name), " "))
}

var (
	storeNameStripRe = regexp.MustCompile(`^(יוחננוף|רמי לוי|שופרסל|קרפור|אושר עד|טיב טעם|קשת|פרשמרקט|פוליצר|סטופ מרקט|סאלח)\s*`)
	branchSuffixRe   = regexp.MustCompile(`(סניף|ישן|חדש|גדול|קטן|המרכזי|מרכז|mall|מול|סטאר סנטר|סנטר|ביג|גרנד|פארק|סיטי|ישפרו|כרמים|נוי|הדר)`)
)

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
