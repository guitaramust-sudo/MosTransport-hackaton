package content

import "testing"

func TestLoadWagonClasses(t *testing.T) {
	classes, err := LoadWagonClasses()
	if err != nil {
		t.Fatal(err)
	}
	if cfg := classes["standard"]; cfg.Status != "available" || len(cfg.SeatAnchors) != 6 || cfg.MoveDurationS != 3 {
		t.Fatalf("standard: %+v", cfg)
	}
	for _, id := range []string{"comfort", "business"} {
		if classes[id].Status != "coming_soon" {
			t.Fatalf("%s should be coming_soon", id)
		}
	}
	first := classes["first"]
	if first.Status == "coming_soon" || first.Status == "" {
		t.Fatalf("first should be a real config, got %+v", first)
	}
	foundServicePoint := false
	for _, a := range first.Anchors {
		if a == first.ServicePointAnchor {
			foundServicePoint = true
		}
	}
	if !foundServicePoint {
		t.Fatalf("first: service_point_anchor %q not in anchors %+v", first.ServicePointAnchor, first.Anchors)
	}
	if len(first.RestrictedAnchors) != 1 || first.RestrictedAnchors[0] != "cab_entrance_boundary" {
		t.Fatalf("first: expected exactly restricted anchor cab_entrance_boundary, got %+v", first.RestrictedAnchors)
	}
}

func TestParseWagonClassesRejectsIncompleteConfig(t *testing.T) {
	if _, err := ParseWagonClasses([]byte(`{"standard":{"anchors":["seat_1"]}}`)); err == nil {
		t.Fatal("incomplete config accepted")
	}
	if _, err := ParseWagonClasses([]byte(`{"comfort":{"status":"coming_soon"}}`)); err != nil {
		t.Fatal(err)
	}
}
