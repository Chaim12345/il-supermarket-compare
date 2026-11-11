// Package api turns a branch's raw price rows into catalogue matches and a
// per-basket comparison.
package api

import (
	"sort"

	"il-supermarket-web/internal/catalog"
	"il-supermarket-web/internal/portals"
	"il-supermarket-web/internal/store"
)

// MatchStore maps a branch's full item list onto the catalogue, keeping the
// best candidate per item. Weighted items are kept at their per-kg price.
func MatchStore(items []portals.Item) []store.PriceRow {
	type cand struct {
		row   store.PriceRow
		score int
	}
	best := map[string]cand{}
	for _, it := range items {
		name := it.Name
		kind := it.Form()
		for i := range catalog.Items {
			ci := &catalog.Items[i]
			if !ci.Match(name) {
				continue
			}
			// A per-kg item must come from loose produce, never a sealed pack
			// (a can of diced tomatoes is not a tomato).
			if ci.Unit == catalog.UnitKg && kind != portals.Loose {
				continue
			}
			// A pack item must carry an accepted pack size, so a store is not
			// credited with a smaller or larger pack at the wrong price.
			if ci.PackLabel != "" && kind == portals.Packaged && !ci.MatchPack(name) {
				continue
			}
			// Osher Ad labels per-kg items with unit "יחידה" while bIsWeighted=1,
			// so the weighted flag alone decides whether the price is per kg.
			row := store.PriceRow{
				ItemID:      ci.ID,
				Price:       it.Price,
				Weighted:    it.Weighted,
				PackQty:     it.Quantity,
				ProductName: name,
			}
			score := ci.PreferScore(name, it.Quantity)
			cur, ok := best[ci.ID]
			if !ok || better(score, it.Price, cur.score, cur.row.Price) {
				best[ci.ID] = cand{row: row, score: score}
			}
		}
	}
	out := make([]store.PriceRow, 0, len(best))
	for _, c := range best {
		out = append(out, c.row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out
}

// better: higher preference score wins; on a tie, the cheaper price wins.
func better(score int, price float64, curScore int, curPrice float64) bool {
	if score != curScore {
		return score > curScore
	}
	return price < curPrice
}

// BasketLine is one catalogue item as requested by the user.
type BasketLine struct {
	ItemID string  `json:"itemId"`
	Qty    float64 `json:"qty"`
}

// StoreResult is one branch's outcome for the whole basket.
type StoreResult struct {
	StoreID string             `json:"storeId"`
	Chain   string             `json:"chain"`
	Name    string             `json:"name"`
	Address string             `json:"address"`
	City    string             `json:"city"`
	Km      float64            `json:"km"`
	Approx  bool               `json:"approx"`
	Total   float64            `json:"total"`
	Found   int                `json:"found"`
	Lines   map[string]LineRes `json:"lines"`
	Missing []string           `json:"missing"`
	Stale   bool               `json:"stale"`
}

// LineRes is the price of one item at one branch.
type LineRes struct {
	Price   float64 `json:"price"`
	Product string  `json:"product"`
	Unit    string  `json:"unit"`
}

// Compare builds per-store results for the basket.
func Compare(stores []store.StoreRow, prices map[string]map[string]store.PriceRow, lines []BasketLine) []StoreResult {
	out := make([]StoreResult, 0, len(stores))
	for _, s := range stores {
		byItem := prices[s.ID]
		res := StoreResult{
			StoreID: s.ID, Chain: s.Chain, Name: s.Name, Address: s.Address,
			City: s.City, Km: 0, Approx: s.Approx, Lines: map[string]LineRes{},
			Missing: []string{},
		}
		for _, line := range lines {
			ci := catalogByID(line.ItemID)
			if ci == nil {
				continue
			}
			row, ok := byItem[line.ItemID]
			if !ok {
				res.Missing = append(res.Missing, line.ItemID)
				continue
			}
			qty := line.Qty
			if qty <= 0 {
				qty = ci.DefaultQty
			}
			// Weighted items are priced per kg and pack items per unit; in both
			// cases the line cost is price x quantity.
			total := row.Price * qty
			unit := "יח'"
			if ci.Unit == catalog.UnitKg || row.Weighted {
				unit = "ק\"ג"
			}
			res.Lines[line.ItemID] = LineRes{Price: round2(total), Product: row.ProductName, Unit: unit}
			res.Total += total
			res.Found++
		}
		res.Total = round2(res.Total)
		out = append(out, res)
	}
	// A store that is missing items is not comparable on price alone, so rank
	// by how much of the basket it covers, then by price.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Found != out[j].Found {
			return out[i].Found > out[j].Found
		}
		return out[i].Total < out[j].Total
	})
	return out
}

func catalogByID(id string) *catalog.Item {
	for i := range catalog.Items {
		if catalog.Items[i].ID == id {
			return &catalog.Items[i]
		}
	}
	return nil
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
