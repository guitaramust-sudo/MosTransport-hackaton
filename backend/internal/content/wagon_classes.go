package content

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed wagon_classes.json
var wagonClassFiles embed.FS

type WagonClassConfig struct {
	Status                  string   `json:"status,omitempty"`
	Anchors                 []string `json:"anchors,omitempty"`
	RestrictedAnchors       []string `json:"restricted_anchors,omitempty"`
	SeatAnchors             []string `json:"seat_anchors,omitempty"`
	MinPassengers           int      `json:"min_passengers,omitempty"`
	MaxPassengers           int      `json:"max_passengers,omitempty"`
	ServicePointAnchor      string   `json:"service_point_anchor,omitempty"`
	SessionDurationS        int      `json:"session_duration_s,omitempty"`
	TickS                   int      `json:"tick_s,omitempty"`
	SpawnCheckIntervalS     int      `json:"spawn_check_interval_s,omitempty"`
	SpawnProbability        float64  `json:"spawn_probability,omitempty"`
	MaxConcurrentSituations int      `json:"max_concurrent_situations,omitempty"`
	MoveDurationS           int      `json:"move_duration_s,omitempty"`
	RedirectDurationS       int      `json:"redirect_duration_s,omitempty"`
}

type WagonClasses map[string]WagonClassConfig

func LoadWagonClasses() (WagonClasses, error) {
	raw, err := wagonClassFiles.ReadFile("wagon_classes.json")
	if err != nil {
		return nil, err
	}
	classes, err := ParseWagonClasses(raw)
	if err != nil {
		return nil, err
	}
	return classes, nil
}

func ParseWagonClasses(raw []byte) (WagonClasses, error) {
	var classes WagonClasses
	if err := json.Unmarshal(raw, &classes); err != nil {
		return nil, fmt.Errorf("wagon_classes.json: %w", err)
	}
	for id, cfg := range classes {
		if cfg.Status == "coming_soon" {
			continue
		}
		if cfg.Status != "" && cfg.Status != "available" {
			return nil, fmt.Errorf("wagon class %q: invalid status", id)
		}
		anchors := map[string]bool{}
		for _, a := range cfg.Anchors {
			if a == "" || anchors[a] {
				return nil, fmt.Errorf("wagon class %q: duplicate or empty anchor", id)
			}
			anchors[a] = true
		}
		if len(cfg.SeatAnchors) == 0 || !anchors[cfg.ServicePointAnchor] {
			return nil, fmt.Errorf("wagon class %q: seat anchors and service point required", id)
		}
		seats := map[string]bool{}
		for _, a := range cfg.SeatAnchors {
			if !anchors[a] || seats[a] {
				return nil, fmt.Errorf("wagon class %q: invalid seat anchor %q", id, a)
			}
			seats[a] = true
		}
		if (cfg.MinPassengers == 0) != (cfg.MaxPassengers == 0) || cfg.MinPassengers < 0 || cfg.MaxPassengers < cfg.MinPassengers || cfg.MaxPassengers > len(cfg.SeatAnchors) {
			return nil, fmt.Errorf("wagon class %q: passenger range must fit the seat anchors", id)
		}
		for _, a := range cfg.RestrictedAnchors {
			if !anchors[a] {
				return nil, fmt.Errorf("wagon class %q: invalid restricted anchor %q", id, a)
			}
		}
		if cfg.SessionDurationS <= 0 || cfg.TickS <= 0 || cfg.SpawnCheckIntervalS <= 0 || cfg.MaxConcurrentSituations <= 0 || cfg.MoveDurationS <= 0 || cfg.RedirectDurationS <= 0 {
			return nil, fmt.Errorf("wagon class %q: timing and concurrency settings must be positive", id)
		}
		if cfg.SpawnProbability < 0 || cfg.SpawnProbability > 1 {
			return nil, fmt.Errorf("wagon class %q: spawn_probability must be in [0,1]", id)
		}
	}
	return classes, nil
}
