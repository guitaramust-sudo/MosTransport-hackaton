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

// PickWagonSpawns considers idle seats only and caps the simultaneous cases.
func PickWagonSpawns(state domain.WagonState, cfg content.WagonClassConfig, scenarios []content.Scenario, elapsed time.Duration, rng *rand.Rand) []WagonSpawnDecision {
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
	fraction := elapsed.Seconds() / float64(cfg.SessionDurationS)
	poolIDs := map[string]bool{}
	for _, id := range cfg.SituationPoolIDs {
		poolIDs[id] = true
	}
	var eligible []content.Scenario
	for _, s := range scenarios {
		if len(poolIDs) > 0 && !poolIDs[s.ID] {
			continue
		}
		if s.ValidationStatus == "blocked" {
			continue
		}
		if fraction >= cfg.PoolUnlock[wagonPool(s.Criticality)] {
			eligible = append(eligible, s)
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].ID < eligible[j].ID })
	var decisions []WagonSpawnDecision
	for i, seat := range state.Seats {
		if active >= cfg.MaxConcurrentSituations {
			break
		}
		if seat.SituationID != nil || rng.Float64() >= cfg.SpawnProbability {
			continue
		}
		chosen := eligible[rng.Intn(len(eligible))]
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
