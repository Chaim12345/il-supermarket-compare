// Package store is the SQLite persistence layer: store locations, a geocode
// cache and a per-day price cache. The price cache is what keeps repeat
// queries instant, since the chain portals are slow and have no CORS.
package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ sql *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS stores (
  id TEXT PRIMARY KEY,
  chain TEXT NOT NULL,
  name TEXT NOT NULL,
  address TEXT NOT NULL DEFAULT '',
  city TEXT NOT NULL DEFAULT '',
  subchain TEXT NOT NULL DEFAULT '',
  lat REAL,
  lon REAL,
  approx INTEGER NOT NULL DEFAULT 0,
  kind TEXT NOT NULL DEFAULT 'regular',
  refreshed TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_stores_chain ON stores(chain);

CREATE TABLE IF NOT EXISTS geocode (
  query TEXT PRIMARY KEY,
  lat REAL,
  lon REAL,
  found INTEGER NOT NULL DEFAULT 0,
  ts TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS price_cache (
  store_id TEXT NOT NULL,
  date TEXT NOT NULL,
  item_id TEXT NOT NULL,
  price REAL NOT NULL,
  weighted INTEGER NOT NULL DEFAULT 0,
  pack_qty REAL NOT NULL DEFAULT 0,
  product_name TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (store_id, date, item_id)
);
CREATE INDEX IF NOT EXISTS idx_price_date ON price_cache(date);

CREATE TABLE IF NOT EXISTS price_meta (
  store_id TEXT NOT NULL,
  date TEXT NOT NULL,
  fetched_at TEXT NOT NULL DEFAULT '',
  item_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (store_id, date)
);

CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`

func Open(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// SQLite allows a single writer; one connection avoids "database is locked".
	d.SetMaxOpenConns(1)
	d.SetConnMaxLifetime(0)
	db := &DB{sql: d}
	if _, err := db.sql.Exec(schema); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error { return db.sql.Close() }

// ---------------- stores ----------------

// StoreRow is a persisted branch, with coordinates resolved at ingest time.
type StoreRow struct {
	ID       string
	Chain    string
	Name     string
	Address  string
	City     string
	SubChain string
	Lat      float64
	Lon      float64
	Approx   bool
	Kind     string
}

func (db *DB) UpsertStores(rows []StoreRow) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO stores
		(id, chain, name, address, city, subchain, lat, lon, approx, kind, refreshed)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
		  chain=excluded.chain, name=excluded.name, address=excluded.address,
		  city=excluded.city, subchain=excluded.subchain, lat=excluded.lat,
		  lon=excluded.lon, approx=excluded.approx, kind=excluded.kind,
		  refreshed=excluded.refreshed`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, r := range rows {
		approx := 0
		if r.Approx {
			approx = 1
		}
		if _, err := stmt.Exec(r.ID, r.Chain, r.Name, r.Address, r.City, r.SubChain,
			r.Lat, r.Lon, approx, r.Kind, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// StoresInBox returns branches whose coordinates fall in the bounding box.
func (db *DB) StoresInBox(minLat, maxLat, minLon, maxLon float64) ([]StoreRow, error) {
	rows, err := db.sql.Query(`SELECT id, chain, name, address, city, subchain, lat, lon, approx, kind
		FROM stores WHERE lat IS NOT NULL AND lon IS NOT NULL
		AND lat BETWEEN ? AND ? AND lon BETWEEN ? AND ?`, minLat, maxLat, minLon, maxLon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoreRow
	for rows.Next() {
		var r StoreRow
		var approx int
		if err := rows.Scan(&r.ID, &r.Chain, &r.Name, &r.Address, &r.City, &r.SubChain,
			&r.Lat, &r.Lon, &approx, &r.Kind); err != nil {
			return nil, err
		}
		r.Approx = approx == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *DB) StoreCount() (int, error) {
	var n int
	err := db.sql.QueryRow(`SELECT COUNT(*) FROM stores`).Scan(&n)
	return n, err
}

// ---------------- geocode cache ----------------

func (db *DB) GeocodeGet(query string) (lat, lon float64, found bool, ok bool) {
	var f int
	err := db.sql.QueryRow(`SELECT lat, lon, found FROM geocode WHERE query = ?`, query).Scan(&lat, &lon, &f)
	if err != nil {
		return 0, 0, false, false
	}
	return lat, lon, f == 1, true
}

func (db *DB) GeocodePut(query string, lat, lon float64, found bool) error {
	f := 0
	if found {
		f = 1
	}
	_, err := db.sql.Exec(`INSERT INTO geocode (query, lat, lon, found, ts) VALUES (?,?,?,?,?)
		ON CONFLICT(query) DO UPDATE SET lat=excluded.lat, lon=excluded.lon, found=excluded.found, ts=excluded.ts`,
		query, lat, lon, f, time.Now().UTC().Format(time.RFC3339))
	return err
}

// ---------------- price cache ----------------

// PriceRow is one catalogue item as priced at one branch.
type PriceRow struct {
	StoreID     string
	ItemID      string
	Price       float64 // per kg when weighted
	Weighted    bool
	PackQty     float64
	ProductName string
}

func (db *DB) HasPrices(storeID, date string) (bool, error) {
	var n int
	err := db.sql.QueryRow(`SELECT COUNT(*) FROM price_meta WHERE store_id = ? AND date = ?`, storeID, date).Scan(&n)
	return n > 0, err
}

func (db *DB) SavePrices(storeID, date string, rows []PriceRow) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM price_cache WHERE store_id = ? AND date = ?`, storeID, date); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO price_cache
		(store_id, date, item_id, price, weighted, pack_qty, product_name) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		w := 0
		if r.Weighted {
			w = 1
		}
		if _, err := stmt.Exec(storeID, date, r.ItemID, r.Price, w, r.PackQty, r.ProductName); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO price_meta (store_id, date, fetched_at, item_count) VALUES (?,?,?,?)`,
		storeID, date, time.Now().UTC().Format(time.RFC3339), len(rows)); err != nil {
		return err
	}
	return tx.Commit()
}

// PricesForStores loads the cached catalogue prices for the given branches.
func (db *DB) PricesForStores(storeIDs []string, date string) (map[string]map[string]PriceRow, error) {
	out := map[string]map[string]PriceRow{}
	if len(storeIDs) == 0 {
		return out, nil
	}
	q := `SELECT store_id, item_id, price, weighted, pack_qty, product_name FROM price_cache WHERE date = ? AND store_id IN (`
	args := []any{date}
	for i, id := range storeIDs {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, id)
	}
	q += ")"
	rows, err := db.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r PriceRow
		var w int
		if err := rows.Scan(&r.StoreID, &r.ItemID, &r.Price, &w, &r.PackQty, &r.ProductName); err != nil {
			return nil, err
		}
		r.Weighted = w == 1
		if out[r.StoreID] == nil {
			out[r.StoreID] = map[string]PriceRow{}
		}
		out[r.StoreID][r.ItemID] = r
	}
	return out, rows.Err()
}

func (db *DB) PruneOldPrices(keepDate string) error {
	_, err := db.sql.Exec(`DELETE FROM price_cache WHERE date <> ?`, keepDate)
	if err != nil {
		return err
	}
	_, err = db.sql.Exec(`DELETE FROM price_meta WHERE date <> ?`, keepDate)
	return err
}

// ---------------- meta ----------------

func (db *DB) MetaGet(key string) (string, error) {
	var v string
	err := db.sql.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (db *DB) MetaSet(key, value string) error {
	_, err := db.sql.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
