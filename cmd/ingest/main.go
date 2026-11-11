// Command ingest refreshes store locations and coordinates into SQLite.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"il-supermarket-web/internal/geo"
	"il-supermarket-web/internal/ingest"
	"il-supermarket-web/internal/store"
)

func main() {
	dbPath := flag.String("db", envOr("DB_PATH", "data/prices.db"), "SQLite database path")
	limit := flag.Int("limit", 0, "max stores to geocode (0 = all)")
	chains := flag.String("chains", "", "comma-separated chain keys (default all)")
	snapshot := flag.String("snapshot", "", "also write a static JSON snapshot to this directory")
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var keys []string
	if *chains != "" {
		keys = strings.Split(*chains, ",")
	}
	st, err := ingest.Run(context.Background(), db, geo.New(db), ingest.Options{
		Limit: *limit, Chains: keys, SnapshotDir: *snapshot,
	})
	if err != nil {
		log.Fatalf("ingest: %v", err)
	}
	log.Printf("done: %d chains, %d stores, %d geocoded, %d in db", st.Chains, st.Stored, st.Geocoded, st.TotalInDB)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
