package service

import (
	"errors"
	"math/rand"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
)

var (
	ErrWrongAnchor        = errors.New("player is not at the required anchor")
	ErrItemNotCarried     = errors.New("item is not carried")
	ErrSeatNotFound       = errors.New("active situation not found in wagon")
	ErrAlreadyMoving      = errors.New("player is still moving")
	ErrInvalidWagonAction = errors.New("invalid wagon action")
)

type WagonArrival struct {
	SeatIndex         int
	Anchor            string
	ReachedRestricted bool
}

type WagonSpawnDecision struct {
	SeatIndex  int
	ScenarioID string
}

func wagonPool(c content.Criticality) string {
	switch c {
	case content.CritCritical:
		return "hard"
	case content.CritHigh:
		return "medium"
	default:
		return "easy"
	}
}

func isRestrictedAnchor(restricted []string, anchor string) bool {
	for _, a := range restricted {
		if a == anchor {
			return true
		}
	}
	return false
}

// AdvanceWagonMovements returns a fresh state, resolving server-timed moves.
func AdvanceWagonMovements(state domain.WagonState, now time.Time) (domain.WagonState, []WagonArrival) {
	state.Seats = append([]domain.WagonSeat(nil), state.Seats...)
	if move := state.Player.Moving; move != nil && !now.Before(move.StartedAt.Add(time.Duration(move.DurationS)*time.Second)) {
		state.Player.At, state.Player.Moving = move.To, nil
	}
	var arrivals []WagonArrival
	for i := range state.Seats {
		move := state.Seats[i].Actor.Moving
		if move == nil || now.Before(move.StartedAt.Add(time.Duration(move.DurationS)*time.Second)) {
			continue
		}
		state.Seats[i].Actor.At, state.Seats[i].Actor.Moving = move.To, nil
		restricted := isRestrictedAnchor(state.RestrictedAnchors, move.To)
		if restricted {
			state.Seats[i].RestrictedReached = true
		}
		arrivals = append(arrivals, WagonArrival{SeatIndex: i, Anchor: move.To, ReachedRestricted: restricted})
	}
	return state, arrivals
}

// PickWagonSpawns considers idle seats only, caps the simultaneous cases, and
// samples eligible scenarios weighted by the active level's per-type weights:
// first pick a scenario Type (weighted by level.TypeWeights), then pick
// uniformly among eligible scenarios of that type. Deterministic given rng.
func PickWagonSpawns(state domain.WagonState, cfg content.WagonClassConfig, level content.Level, scenarios []content.Scenario, elapsed time.Duration, rng *rand.Rand) []WagonSpawnDecision {
	if cfg.SessionDurationS <= 0 || elapsed >= time.Duration(cfg.SessionDurationS)*time.Second || cfg.MaxConcurrentSituations <= 0 {
		return nil
	}
	active := 0
	for _, seat := range state.Seats {
		if seat.SituationID != nil {
			active++
		}
	}
	if active >= cfg.MaxConcurrentSituations {
		return nil
	}
	byType := map[string][]content.Scenario{}
	for _, s := range scenarios {
		if s.ValidationStatus == "blocked" {
			continue
		}
		if level.TypeWeights[string(s.Type)] <= 0 {
			continue
		}
		byType[string(s.Type)] = append(byType[string(s.Type)], s)
	}
	if len(byType) == 0 {
		return nil
	}
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types) // map iteration order is random; sort so rng draws are reproducible
	totalWeight := 0.0
	for _, t := range types {
		sort.Slice(byType[t], func(i, j int) bool { return byType[t][i].ID < byType[t][j].ID })
		totalWeight += level.TypeWeights[t]
	}

	pickScenario := func() content.Scenario {
		roll := rng.Float64() * totalWeight
		for _, t := range types {
			w := level.TypeWeights[t]
			if roll < w {
				pool := byType[t]
				return pool[rng.Intn(len(pool))]
			}
			roll -= w
		}
		last := byType[types[len(types)-1]]
		return last[rng.Intn(len(last))]
	}

	var decisions []WagonSpawnDecision
	for i, seat := range state.Seats {
		if active >= cfg.MaxConcurrentSituations {
			break
		}
		if seat.SituationID != nil || rng.Float64() >= cfg.SpawnProbability {
			continue
		}
		chosen := pickScenario()
		decisions = append(decisions, WagonSpawnDecision{SeatIndex: i, ScenarioID: chosen.ID})
		active++
	}
	return decisions
}

func requireStationary(state domain.WagonState) error {
	if state.Player.Moving != nil {
		return ErrAlreadyMoving
	}
	return nil
}

func ApplyWagonMove(state domain.WagonState, destination string, cfg content.WagonClassConfig, now time.Time) (domain.WagonState, error) {
	if err := requireStationary(state); err != nil {
		return state, err
	}
	valid := false
	for _, a := range cfg.Anchors {
		if a == destination {
			valid = true
			break
		}
	}
	if !valid || state.Player.At == destination {
		return state, ErrInvalidWagonAction
	}
	state.Player.Moving = &domain.WagonMove{From: state.Player.At, To: destination, StartedAt: now, DurationS: cfg.MoveDurationS}
	return state, nil
}

func ApplyPickItem(state domain.WagonState, servicePoint, item string) (domain.WagonState, error) {
	if err := requireStationary(state); err != nil {
		return state, err
	}
	if state.Player.At != servicePoint {
		return state, ErrWrongAnchor
	}
	if item == "" {
		return state, ErrInvalidWagonAction
	}
	state.CarriedItems = append(append([]string(nil), state.CarriedItems...), item)
	return state, nil
}

func findSeatBySituation(state domain.WagonState, situationID uuid.UUID) int {
	for i, seat := range state.Seats {
		if seat.SituationID != nil && *seat.SituationID == situationID {
			return i
		}
	}
	return -1
}

func ApplyGiveItem(state domain.WagonState, situationID uuid.UUID, item string) (domain.WagonState, error) {
	if err := requireStationary(state); err != nil {
		return state, err
	}
	idx := findSeatBySituation(state, situationID)
	if idx < 0 {
		return state, ErrSeatNotFound
	}
	if state.Player.At != state.Seats[idx].Actor.At {
		return state, ErrWrongAnchor
	}
	pos := -1
	for i, carried := range state.CarriedItems {
		if carried == item {
			pos = i
			break
		}
	}
	if pos < 0 {
		return state, ErrItemNotCarried
	}
	items := append([]string(nil), state.CarriedItems[:pos]...)
	state.CarriedItems = append(items, state.CarriedItems[pos+1:]...)
	return state, nil
}

func ApplyRedirect(state domain.WagonState, situationID uuid.UUID) (domain.WagonState, error) {
	if err := requireStationary(state); err != nil {
		return state, err
	}
	idx := findSeatBySituation(state, situationID)
	if idx < 0 {
		return state, ErrSeatNotFound
	}
	seat := state.Seats[idx]
	if state.Player.At != seat.Actor.At {
		return state, ErrWrongAnchor
	}
	state.Seats = append([]domain.WagonSeat(nil), state.Seats...)
	state.Seats[idx].Actor.At = seat.Anchor
	state.Seats[idx].Actor.Moving = nil
	return state, nil
}
