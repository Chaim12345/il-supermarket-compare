package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"il-supermarket-web/internal/api"
	"il-supermarket-web/internal/catalog"
	"il-supermarket-web/internal/geo"
	"il-supermarket-web/internal/ingest"
	"il-supermarket-web/internal/portals"
	"il-supermarket-web/internal/store"
)

type server struct {
	db     *store.DB
	geo    *geo.Geocoder
	static string
}

func main() {
	dbPath := flag.String("db", envOr("DB_PATH", "data/prices.db"), "SQLite database path")
	addr := flag.String("addr", envOr("ADDR", ":8787"), "listen address")
	static := flag.String("static", envOr("STATIC_DIR", "web/dist"), "frontend build directory")
	selfRefresh := flag.Bool("self-refresh", envOr("SELF_REFRESH", "1") == "1", "refresh store list daily")
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	srv := &server{db: db, geo: geo.New(db), static: *static}
	// Yesterday's prices are meaningless for comparison; drop them at boot.
	if err := db.PruneOldPrices(israelDate()); err != nil {
		log.Printf("prune: %v", err)
	}
	if *selfRefresh {
		go srv.refreshLoop()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", srv.health)
	mux.HandleFunc("GET /api/meta", srv.meta)
	mux.HandleFunc("GET /api/stores", srv.stores)
	mux.HandleFunc("POST /api/compare", srv.compare)
	mux.Handle("/", srv.frontend())

	log.Printf("listening on %s (db=%s, static=%s)", *addr, *dbPath, *static)
	if err := http.ListenAndServe(*addr, cors(mux)); err != nil {
		log.Fatal(err)
	}
}

// cors allows the UI to be hosted on another origin (e.g. GitHub Pages) while
// the API runs elsewhere. The API is read-only and public, so "*" is safe.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// refreshLoop keeps the store list and coordinates current. On first boot with
// an empty database it runs immediately; afterwards it runs daily.
func (s *server) refreshLoop() {
	if n, err := s.db.StoreCount(); err == nil && n == 0 {
		log.Printf("self-refresh: empty store table, ingesting now (this takes a few minutes)")
		if _, err := ingest.Run(context.Background(), s.db, s.geo, ingest.Options{}); err != nil {
			log.Printf("self-refresh: %v", err)
		}
	}
	for {
		time.Sleep(24 * time.Hour)
		if _, err := ingest.Run(context.Background(), s.db, s.geo, ingest.Options{}); err != nil {
			log.Printf("self-refresh: %v", err)
		}
		// Keep only today's cached prices.
		if err := s.db.PruneOldPrices(israelDate()); err != nil {
			log.Printf("prune: %v", err)
		}
	}
}

func israelDate() string {
	loc, err := time.LoadLocation("Asia/Jerusalem")
	if err != nil {
		loc = time.FixedZone("IST", 2*3600)
	}
	return time.Now().In(loc).Format("20060102")
}

func (s *server) frontend() http.Handler {
	fs := http.FileServer(http.Dir(s.static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(s.static, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(p); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(s.static, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	n, _ := s.db.StoreCount()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stores": n, "date": israelDate()})
}

func (s *server) meta(w http.ResponseWriter, _ *http.Request) {
	type chainOut struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	chains := make([]chainOut, 0, len(portals.Chains))
	for _, c := range portals.Chains {
		chains = append(chains, chainOut{Key: c.Key, Name: c.Name, Kind: string(c.Kind)})
	}
	gen, _ := s.db.MetaGet("stores_refreshed")
	writeJSON(w, http.StatusOK, map[string]any{
		"catalog":     catalog.Items,
		"chains":      chains,
		"generatedAt": gen,
		"priceDate":   israelDate(),
	})
}

func (s *server) stores(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat := atof(q.Get("lat"))
	lon := atof(q.Get("lon"))
	radius := atofDefault(q.Get("radius"), 4)
	if lat == 0 && lon == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "lat/lon required"})
		return
	}
	minLat, maxLat, minLon, maxLon := geo.BoundingBox(lat, lon, radius)
	rows, err := s.db.StoresInBox(minLat, maxLat, minLon, maxLon)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type out struct {
		store.StoreRow
		Km float64 `json:"km"`
	}
	var res []out
	for _, sr := range rows {
		if d := geo.HaversineKm(lat, lon, sr.Lat, sr.Lon); d <= radius {
			res = append(res, out{sr, round2(d)})
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Km < res[j].Km })
	writeJSON(w, http.StatusOK, res)
}

type compareRequest struct {
	Address   string           `json:"address"`
	RadiusKm  float64          `json:"radiusKm"`
	Items     []api.BasketLine `json:"items"`
	MaxStores int              `json:"maxStores"`
}

type near struct {
	row store.StoreRow
	km  float64
}

func (s *server) compare(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 110*time.Second)
	defer cancel()

	var req compareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON"})
		return
	}
	if strings.TrimSpace(req.Address) == "" || len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "address and items are required"})
		return
	}
	if req.RadiusKm <= 0 {
		req.RadiusKm = 4
	}
	if req.MaxStores <= 0 {
		req.MaxStores = 10
	}

	lat, lon, found, err := s.geo.Lookup(ctx, req.Address)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "geocoding failed: " + err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{"error": "address not found", "results": []any{}})
		return
	}

	minLat, maxLat, minLon, maxLon := geo.BoundingBox(lat, lon, req.RadiusKm)
	rows, err := s.db.StoresInBox(minLat, maxLat, minLon, maxLon)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var inRadius []near
	for _, sr := range rows {
		d := geo.HaversineKm(lat, lon, sr.Lat, sr.Lon)
		limit := req.RadiusKm
		if sr.Approx {
			limit = req.RadiusKm - 1.5 // a city-centre fallback is imprecise
		}
		if d <= limit {
			inRadius = append(inRadius, near{sr, d})
		}
	}
	sort.Slice(inRadius, func(i, j int) bool { return inRadius[i].km < inRadius[j].km })

	targets := selectTargets(inRadius, req.MaxStores)
	date := israelDate()
	skipped := []string{}
	priceMap, err := s.ensurePrices(ctx, targets, date, &skipped)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	storeRows := make([]store.StoreRow, 0, len(targets))
	for _, t := range targets {
		storeRows = append(storeRows, t.row)
	}
	results := api.Compare(storeRows, priceMap, req.Items)
	kmByID := map[string]float64{}
	for _, t := range targets {
		kmByID[t.row.ID] = t.km
	}
	for i := range results {
		results[i].Km = round2(kmByID[results[i].StoreID])
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"origin":   map[string]any{"lat": lat, "lon": lon, "label": req.Address},
		"date":     date,
		"results":  results,
		"summary":  summarize(results, req.Items),
		"skipped":  skipped,
		"radiusKm": req.RadiusKm,
	})
}

// selectTargets keeps the branches worth pricing: every discount branch in
// range (they are priced per branch) plus the nearest branch of each other
// banner, capped so a request stays fast.
func selectTargets(cands []near, max int) []near {
	seenRegular := map[string]int{}
	var out []near
	for _, c := range cands {
		chain, ok := portals.ChainByKey(c.row.Chain)
		if !ok {
			continue
		}
		if chain.Kind == portals.KindDiscount {
			out = append(out, c)
		} else if seenRegular[c.row.Chain] < 1 {
			seenRegular[c.row.Chain]++
			out = append(out, c)
		}
		if len(out) >= max {
			break
		}
	}
	return out
}

func (s *server) ensurePrices(ctx context.Context, targets []near, date string, skipped *[]string) (map[string]map[string]store.PriceRow, error) {
	var mu sync.Mutex
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, t := range targets {
		cached, err := s.db.HasPrices(t.row.ID, date)
		if err != nil {
			return nil, err
		}
		if cached {
			continue
		}
		wg.Add(1)
		go func(t near) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			chain, ok := portals.ChainByKey(t.row.Chain)
			if !ok {
				return
			}
			ctx2, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			items, err := portals.FetchPriceFull(ctx2, chain, storeIDOf(t.row.ID))
			if err != nil {
				mu.Lock()
				*skipped = append(*skipped, t.row.ID)
				mu.Unlock()
				log.Printf("price fetch failed %s: %v", t.row.ID, err)
				return
			}
			if err := s.db.SavePrices(t.row.ID, date, api.MatchStore(items)); err != nil {
				log.Printf("save prices %s: %v", t.row.ID, err)
			}
		}(t)
	}
	wg.Wait()

	ids := make([]string, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, t.row.ID)
	}
	return s.db.PricesForStores(ids, date)
}

func storeIDOf(id string) string {
	if i := strings.LastIndex(id, ":"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// summarize builds the like-for-like and cherry-pick numbers.
func summarize(results []api.StoreResult, lines []api.BasketLine) map[string]any {
	common := map[string]bool{}
	first := true
	for _, r := range results {
		if len(r.Lines) == 0 {
			continue
		}
		have := map[string]bool{}
		for id := range r.Lines {
			have[id] = true
		}
		if first {
			common = have
			first = false
			continue
		}
		for id := range common {
			if !have[id] {
				delete(common, id)
			}
		}
	}

	type lfl struct {
		StoreID string  `json:"storeId"`
		Name    string  `json:"name"`
		Total   float64 `json:"total"`
	}
	var likeForLike []lfl
	for _, r := range results {
		if len(r.Lines) == 0 {
			continue
		}
		total, ok := 0.0, true
		for id := range common {
			l, present := r.Lines[id]
			if !present {
				ok = false
				break
			}
			total += l.Price
		}
		if ok {
			likeForLike = append(likeForLike, lfl{r.StoreID, r.Name, round2(total)})
		}
	}
	sort.Slice(likeForLike, func(i, j int) bool { return likeForLike[i].Total < likeForLike[j].Total })

	cherry, missing := 0.0, 0
	for _, line := range lines {
		best := 0.0
		for _, r := range results {
			if l, ok := r.Lines[line.ItemID]; ok && (best == 0 || l.Price < best) {
				best = l.Price
			}
		}
		if best > 0 {
			cherry += best
		} else {
			missing++
		}
	}

	return map[string]any{
		"likeForLike":   likeForLike,
		"commonCount":   len(common),
		"cherryPick":    round2(cherry),
		"cherryMissing": missing,
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func atof(s string) float64 { return atofDefault(s, 0) }

func atofDefault(s string, def float64) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return def
	}
	return f
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
