package store

import (
	"path/filepath"
	"testing"
)

func TestPriceCacheRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if ok, _ := db.HasPrices("shufersal:241", "20261111"); ok {
		t.Fatal("empty cache reported a hit")
	}
	rows := []PriceRow{
		{StoreID: "shufersal:241", ItemID: "milk", Price: 6.2, ProductName: "חלב 3%"},
		{StoreID: "shufersal:241", ItemID: "tomato", Price: 7.9, Weighted: true, ProductName: "עגבניה"},
	}
	if err := db.SavePrices("shufersal:241", "20261111", rows); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.HasPrices("shufersal:241", "20261111"); !ok {
		t.Fatal("cache miss after save")
	}

	got, err := db.PricesForStores([]string{"shufersal:241"}, "20261111")
	if err != nil {
		t.Fatal(err)
	}
	if len(got["shufersal:241"]) != 2 {
		t.Fatalf("got %d rows", len(got["shufersal:241"]))
	}
	if r := got["shufersal:241"]["tomato"]; !r.Weighted || r.Price != 7.9 {
		t.Errorf("weighted row wrong: %+v", r)
	}

	// A different day must not reuse today's prices.
	if ok, _ := db.HasPrices("shufersal:241", "20261112"); ok {
		t.Error("prices leaked across dates")
	}
}

func TestStoresInBox(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.UpsertStores([]StoreRow{
		{ID: "a:1", Chain: "a", Name: "in", Lat: 32.08, Lon: 34.81},
		{ID: "a:2", Chain: "a", Name: "out", Lat: 31.77, Lon: 35.21},
		{ID: "a:3", Chain: "a", Name: "no coords"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.StoresInBox(32.0, 32.2, 34.7, 34.9)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "in" {
		t.Fatalf("box query wrong: %+v", rows)
	}
	if n, _ := db.StoreCount(); n != 3 {
		t.Errorf("store count = %d", n)
	}
}

func TestPruneOldPrices(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_ = db.SavePrices("a:1", "20261110", []PriceRow{{StoreID: "a:1", ItemID: "milk", Price: 6}})
	_ = db.SavePrices("a:1", "20261111", []PriceRow{{StoreID: "a:1", ItemID: "milk", Price: 7}})
	if err := db.PruneOldPrices("20261111"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.HasPrices("a:1", "20261110"); ok {
		t.Error("yesterday's prices survived the prune")
	}
	if ok, _ := db.HasPrices("a:1", "20261111"); !ok {
		t.Error("today's prices were pruned")
	}
}
