package geo

import (
	"math"
	"testing"
)

func TestHaversine(t *testing.T) {
	// Tel Aviv to Jerusalem is about 54 km.
	d := HaversineKm(32.0853, 34.7818, 31.7683, 35.2137)
	if d < 50 || d > 58 {
		t.Errorf("TA->Jerusalem = %.1f km, want ~54", d)
	}
	if d := HaversineKm(32.0, 34.0, 32.0, 34.0); d != 0 {
		t.Errorf("same point = %v", d)
	}
}

func TestBoundingBox(t *testing.T) {
	minLat, maxLat, minLon, maxLon := BoundingBox(32.08, 34.81, 4)
	if !(minLat < 32.08 && maxLat > 32.08 && minLon < 34.81 && maxLon > 34.81) {
		t.Fatalf("box does not contain the point: %v %v %v %v", minLat, maxLat, minLon, maxLon)
	}
	// The latitude span of a 4 km box is about 0.072 degrees.
	if math.Abs((maxLat-minLat)-0.072) > 0.005 {
		t.Errorf("lat span = %.4f, want ~0.072", maxLat-minLat)
	}
}

func TestParsePhoton(t *testing.T) {
	// Photon returns GeoJSON, coordinates as [lon, lat].
	body := []byte(`{"features":[{"geometry":{"coordinates":[34.7706,32.0668]},
		"properties":{"countrycode":"IL"}}]}`)
	lat, lon, found, err := parsePhoton(body)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if math.Abs(lat-32.0668) > 1e-6 || math.Abs(lon-34.7706) > 1e-6 {
		t.Fatalf("lat/lon swapped: %v %v", lat, lon)
	}
	// A foreign hit must be skipped.
	if _, _, found, _ := parsePhoton([]byte(`{"features":[{"geometry":{"coordinates":[2.35,48.85]},"properties":{"countrycode":"FR"}}]}`)); found {
		t.Error("accepted a non-Israeli result")
	}
}

func TestParseNominatim(t *testing.T) {
	body := []byte(`[{"lat":"32.0668","lon":"34.7706","display_name":"x"}]`)
	lat, lon, found, err := parseNominatim(body)
	if err != nil || !found || math.Abs(lat-32.0668) > 1e-6 || math.Abs(lon-34.7706) > 1e-6 {
		t.Fatalf("parse failed: %v %v %v %v", lat, lon, found, err)
	}
	if _, _, found, _ := parseNominatim([]byte(`[]`)); found {
		t.Error("empty result reported as found")
	}
}
