package content

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed levels.json
var levelFiles embed.FS

// Level is one rung of the sequential wagon-progression ladder. Order is
// 1-based and must be unique and contiguous starting at 1 — levels are a
// linear chain, not a tree. TypeWeights keys must be valid ScenarioType
// values (service/conflict/medical/safety/informational); a level only
// spawns scenarios of the types it lists, weighted relative to each other.
type Level struct {
	ID          string             `json:"id"`
	Order       int                `json:"order"`
	ClassID     string             `json:"class_id"`
	Title       string             `json:"title"`
	Intro       string             `json:"intro"`
	TypeWeights map[string]float64 `json:"type_weights"`
}

type Levels []Level

func LoadLevels() (Levels, error) {
	raw, err := levelFiles.ReadFile("levels.json")
	if err != nil {
		return nil, err
	}
	catalog, err := Load()
	if err != nil {
		return nil, err
	}
	classes, err := LoadWagonClasses()
	if err != nil {
		return nil, err
	}
	return ParseLevels(raw, catalog, classes)
}

// ParseLevels also lets tests validate a candidate levels.json against a
// candidate catalog/classes before embedding it in a build.
func ParseLevels(raw []byte, catalog Catalog, classes WagonClasses) (Levels, error) {
	var levels Levels
	if err := json.Unmarshal(raw, &levels); err != nil {
		return nil, fmt.Errorf("levels.json: %w", err)
	}
	if len(levels) == 0 {
		return nil, fmt.Errorf("levels.json: must define at least one level")
	}
	validTypes := map[string]bool{
		string(TypeService): true, string(TypeConflict): true, string(TypeMedical): true,
		string(TypeSafety): true, string(TypeInformational): true,
	}
	ids := map[string]bool{}
	orders := map[int]bool{}
	sorted := append(Levels(nil), levels...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Order < sorted[j].Order })
	for i, lvl := range sorted {
		where := fmt.Sprintf("level[%d]", i)
		if lvl.ID == "" || ids[lvl.ID] {
			return nil, fmt.Errorf("%s: empty or duplicate id %q", where, lvl.ID)
		}
		ids[lvl.ID] = true
		if lvl.Order != i+1 {
			return nil, fmt.Errorf("%s (id %q): order must be contiguous starting at 1, got %d at position %d", where, lvl.ID, lvl.Order, i+1)
		}
		if orders[lvl.Order] {
			return nil, fmt.Errorf("%s: duplicate order %d", where, lvl.Order)
		}
		orders[lvl.Order] = true
		if lvl.Title == "" || lvl.Intro == "" {
			return nil, fmt.Errorf("%s (id %q): title and intro are required", where, lvl.ID)
		}
		cfg, ok := classes[lvl.ClassID]
		if !ok || cfg.Status == "coming_soon" {
			return nil, fmt.Errorf("%s (id %q): class_id %q is not a playable wagon class", where, lvl.ID, lvl.ClassID)
		}
		if len(lvl.TypeWeights) == 0 {
			return nil, fmt.Errorf("%s (id %q): type_weights must be non-empty", where, lvl.ID)
		}
		anyPositive := false
		for typ, weight := range lvl.TypeWeights {
			if !validTypes[typ] {
				return nil, fmt.Errorf("%s (id %q): invalid scenario type %q in type_weights", where, lvl.ID, typ)
			}
			if weight < 0 {
				return nil, fmt.Errorf("%s (id %q): negative weight for type %q", where, lvl.ID, typ)
			}
			if weight > 0 {
				anyPositive = true
			}
		}
		if !anyPositive {
			return nil, fmt.Errorf("%s (id %q): type_weights must have at least one positive weight", where, lvl.ID)
		}
	}
	_ = catalog // catalog is accepted for future content-existence checks; not used yet
	return sorted, nil
}
