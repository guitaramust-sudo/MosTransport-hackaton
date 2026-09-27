package content

import "testing"

func TestLoadWagonClasses(t *testing.T) {
	classes, err := LoadWagonClasses()
	if err != nil {
		t.Fatal(err)
	}
	if cfg := classes["standard"]; cfg.Status != "available" || len(cfg.SeatAnchors) != 12 || cfg.MinPassengers != 5 || cfg.MaxPassengers != 12 || cfg.MoveDurationS != 3 {
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
	if len(first.SeatAnchors) != 12 || first.MinPassengers != 5 || first.MaxPassengers != 12 {
		t.Fatalf("first: invalid passenger capacity: %+v", first)
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

func TestParseWagonClassesRejectsPassengerRangeBeyondSeats(t *testing.T) {
	for _, passengerRange := range []string{
		`"min_passengers":2,"max_passengers":3`,
		`"min_passengers":2`,
		`"min_passengers":3,"max_passengers":2`,
	} {
		raw := []byte(`{"standard":{"anchors":["seat_1","seat_2","service_point"],"seat_anchors":["seat_1","seat_2"],"service_point_anchor":"service_point","session_duration_s":480,"tick_s":1,"spawn_check_interval_s":3,"max_concurrent_situations":1,"move_duration_s":3,"redirect_duration_s":20,` + passengerRange + `}}`)
		if _, err := ParseWagonClasses(raw); err == nil {
			t.Fatalf("invalid passenger range accepted: %s", passengerRange)
		}
	}
}
