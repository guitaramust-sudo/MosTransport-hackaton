package content

import "testing"

func TestLoadLevels(t *testing.T) {
	levels, err := LoadLevels()
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) < 2 {
		t.Fatalf("expected at least 2 levels, got %d", len(levels))
	}
	for i, lvl := range levels {
		if lvl.Order != i+1 {
			t.Fatalf("levels must be sorted and contiguous by order, got %+v at position %d", lvl, i)
		}
	}
}

func TestParseLevelsRejectsNonContiguousOrder(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	classes, err := LoadWagonClasses()
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`[
		{"id":"a","order":1,"class_id":"standard","title":"A","intro":"A","type_weights":{"informational":1.0}},
		{"id":"b","order":3,"class_id":"standard","title":"B","intro":"B","type_weights":{"service":1.0}}
	]`)
	if _, err := ParseLevels(raw, catalog, classes); err == nil {
		t.Fatal("expected an error for non-contiguous order")
	}
}

func TestParseLevelsRejectsUnknownScenarioType(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	classes, err := LoadWagonClasses()
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`[{"id":"a","order":1,"class_id":"standard","title":"A","intro":"A","type_weights":{"nonsense":1.0}}]`)
	if _, err := ParseLevels(raw, catalog, classes); err == nil {
		t.Fatal("expected an error for an unknown scenario type")
	}
}

func TestParseLevelsRejectsNonPlayableClass(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	classes, err := LoadWagonClasses()
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`[{"id":"a","order":1,"class_id":"comfort","title":"A","intro":"A","type_weights":{"service":1.0}}]`)
	if _, err := ParseLevels(raw, catalog, classes); err == nil {
		t.Fatal("expected an error for a coming_soon class_id")
	}
}
