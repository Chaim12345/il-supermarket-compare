package api

import (
	"testing"

	"il-supermarket-web/internal/portals"
	"il-supermarket-web/internal/store"
)

func TestMatchStorePicksStandardPack(t *testing.T) {
	items := []portals.Item{
		{Name: "גבינה צהובה 250 גרם", Price: 22.0, Quantity: 0.25},
		{Name: "גבינה צהובה 400 גרם", Price: 34.0, Quantity: 0.4},
		{Name: "גבינה צהובה מגורדת", Price: 18.0, Quantity: 0.2},
		{Name: "מלפפון", Price: 5.0, Quantity: 1, Weighted: true},
	}
	rows := MatchStore(items)
	byID := map[string]store.PriceRow{}
	for _, r := range rows {
		byID[r.ItemID] = r
	}
	if c, ok := byID["yellow-cheese"]; !ok {
		t.Fatal("yellow-cheese not matched")
	} else if c.ProductName != "גבינה צהובה 400 גרם" {
		t.Errorf("picked %q, want the 400 g pack", c.ProductName)
	}
	if _, ok := byID["cucumber"]; !ok {
		t.Error("cucumber not matched")
	}
}

func TestMatchStoreWeightedKeepsPerKg(t *testing.T) {
	rows := MatchStore([]portals.Item{
		{Name: "עגבניה", Price: 6.9, Quantity: 1, Weighted: true, Unit: `ק"ג`},
	})
	if len(rows) != 1 || !rows[0].Weighted || rows[0].Price != 6.9 {
		t.Fatalf("weighted tomato wrong: %+v", rows)
	}
}

// A sealed pack whose name merely contains a produce word must not be priced as
// the loose item: canned diced tomatoes are not tomatoes.
func TestMatchStoreRejectsPackForKgItem(t *testing.T) {
	rows := MatchStore([]portals.Item{
		{Name: "עגבניות קוביות 148 ג", Price: 4.5, Quantity: 0.148},
		{Name: "עגבניה", Price: 6.5, Quantity: 1, Weighted: true},
	})
	for _, r := range rows {
		if r.ItemID == "tomato" && r.ProductName != "עגבניה" {
			t.Fatalf("pack matched the per-kg tomato: %+v", r)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("want exactly the loose tomato, got %+v", rows)
	}
}

// A pack item is only accepted at an accepted pack size.
func TestMatchStoreRequiresPackSize(t *testing.T) {
	rows := MatchStore([]portals.Item{
		{Name: "טונה בשמן 160 גרם", Price: 8.9, Quantity: 0.16},
		{Name: "טונה בשמן 80 גרם", Price: 5.5, Quantity: 0.08},
	})
	if len(rows) != 1 || rows[0].ItemID != "tuna" || rows[0].Price != 8.9 {
		t.Fatalf("pack-size gate failed: %+v", rows)
	}
}

func TestCompareBasket(t *testing.T) {
	stores := []store.StoreRow{
		{ID: "a:1", Chain: "a", Name: "זול"},
		{ID: "b:1", Chain: "b", Name: "יקר"},
	}
	prices := map[string]map[string]store.PriceRow{
		"a:1": {
			"milk":   {ItemID: "milk", Price: 6.0, ProductName: "חלב"},
			"tomato": {ItemID: "tomato", Price: 7.0, Weighted: true, ProductName: "עגבניה"},
		},
		"b:1": {
			"milk": {ItemID: "milk", Price: 7.5, ProductName: "חלב"},
			// tomato missing
		},
	}
	lines := []BasketLine{{ItemID: "milk", Qty: 2}, {ItemID: "tomato", Qty: 1}}
	res := Compare(stores, prices, lines)
	if len(res) != 2 {
		t.Fatalf("got %d results", len(res))
	}
	if res[0].StoreID != "a:1" {
		t.Errorf("cheapest first: got %s", res[0].StoreID)
	}
	// 6.0*2 milk + 7.0*1 tomato
	if res[0].Total != 19.0 {
		t.Errorf("total = %v, want 19", res[0].Total)
	}
	if len(res[1].Missing) != 1 || res[1].Missing[0] != "tomato" {
		t.Errorf("missing = %v", res[1].Missing)
	}
}

// A store with every item found must still report an empty (non-nil) missing
// slice, so the JSON field is [] and not null for the frontend.
func TestCompareMissingNeverNull(t *testing.T) {
	stores := []store.StoreRow{{ID: "a:1", Chain: "a", Name: "x"}}
	prices := map[string]map[string]store.PriceRow{
		"a:1": {"milk": {ItemID: "milk", Price: 6.5, ProductName: "חלב"}},
	}
	res := Compare(stores, prices, []BasketLine{{ItemID: "milk", Qty: 1}})
	if res[0].Missing == nil {
		t.Fatal("Missing is nil; JSON would encode as null")
	}
}
