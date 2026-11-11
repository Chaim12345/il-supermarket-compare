package catalog

import "testing"

func item(t *testing.T, id string) *Item {
	t.Helper()
	for i := range Items {
		if Items[i].ID == id {
			return &Items[i]
		}
	}
	t.Fatalf("no catalog item %q", id)
	return nil
}

func TestMatchFreshProduce(t *testing.T) {
	cases := []struct {
		id, name string
		want     bool
	}{
		{"cucumber", "מלפפון", true},
		{"cucumber", "מלפפון חמוץ", false},
		{"tomato", "עגבניה", true},
		{"tomato", "עגבניות שרי", false},
		{"potato", `תפו"א`, true},
		{"potato", "תפוח אדמה אדום", true},
		{"apple", "תפוח עץ", true},
		{"apple", "תפוח אדמה", false},
		{"onion", "בצל יבש", false},
		{"onion", "בצל", true},
	}
	for _, c := range cases {
		if got := item(t, c.id).Match(c.name); got != c.want {
			t.Errorf("%s.Match(%q) = %v, want %v", c.id, c.name, got, c.want)
		}
	}
}

func TestMatchDairy(t *testing.T) {
	cases := []struct {
		id, name string
		want     bool
	}{
		{"milk", "חלב 3% בקרטון", true},
		{"milk", "חלב שוקולד", false},
		{"milk", "משקה חלב בטעם", false},
		{"yellow-cheese", `גבינה צהובה 28%`, true},
		{"yellow-cheese", "גבינה צהובה מגורדת", false},
		{"eggs", "ביצים L 12 יחידות", true},
		{"eggs", "אטריות ביצים", false},
		{"cottage", "קוטג' 5%", true},
		{"butter", "חמאה 100 גרם", true},
		{"butter", "חמאת בוטנים", false},
	}
	for _, c := range cases {
		if got := item(t, c.id).Match(c.name); got != c.want {
			t.Errorf("%s.Match(%q) = %v, want %v", c.id, c.name, got, c.want)
		}
	}
}

func TestPackPreference(t *testing.T) {
	cheese := item(t, "yellow-cheese")
	standard := cheese.PreferScore("גבינה צהובה 400 גרם", 0.4)
	odd := cheese.PreferScore("גבינה צהובה 250 גרם", 0.25)
	if standard <= odd {
		t.Errorf("400g pack should outrank 250g: %d vs %d", standard, odd)
	}
	if kg := item(t, "cucumber").PreferScore("מלפפון", 1); kg != 0 {
		t.Errorf("weighted items must score 0, got %d", kg)
	}
}
