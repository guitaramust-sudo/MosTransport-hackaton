package content

import "testing"

func TestLoadWagonClasses(t *testing.T) {
	classes, err := LoadWagonClasses()
	if err != nil {
		t.Fatal(err)
	}
	if cfg := classes["standard"]; cfg.Status != "available" || len(cfg.SeatAnchors) != 6 || cfg.MoveDurationS != 3 || len(cfg.SituationPoolIDs) != 54 {
		t.Fatalf("standard: %+v", cfg)
	}
	for _, id := range []string{"comfort", "business", "first"} {
		if classes[id].Status != "coming_soon" {
			t.Fatalf("%s should be coming_soon", id)
		}
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
