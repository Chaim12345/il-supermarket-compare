package ingest

import "testing"

func TestLocalityFromName(t *testing.T) {
	cases := map[string]string{
		"יוחננוף מפוח":      "מפוח",
		"יוחננוף אור יהודה": "אור יהודה",
		"בילו":              "בילו",
		"מודיעין ישפרו":     "מודיעין",
		"ירושלים תלפיות":    "ירושלים תלפיות",
		"יוחננוף סניף ראשי": "ראשי",
		"":                  "",
	}
	for in, want := range cases {
		if got := localityFromName(in); got != want {
			t.Errorf("localityFromName(%q) = %q, want %q", in, got, want)
		}
	}
}
