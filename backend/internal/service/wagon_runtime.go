package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

type wagonStore interface {
	CreateWagonSession(context.Context, uuid.UUID, domain.WagonState) (domain.Session, error)
	UpdateWagonState(context.Context, uuid.UUID, domain.WagonState) error
	ListActiveWagonSessions(context.Context) ([]domain.Session, error)
	CreateSituation(context.Context, domain.Situation) (domain.Situation, error)
	GetSituation(context.Context, uuid.UUID) (domain.Situation, error)
	ListSituationsBySession(context.Context, uuid.UUID) ([]domain.Situation, error)
	SetPhysicalActionDone(context.Context, uuid.UUID) error
	CompleteWagonPhysicalAction(context.Context, uuid.UUID, uuid.UUID, domain.WagonState) error
	RecordRestrictedArrival(context.Context, uuid.UUID) error
	GetSession(context.Context, uuid.UUID) (domain.Session, error)
	GetPlayerByID(context.Context, uuid.UUID) (domain.Player, error)
	AdvanceWagonProgress(context.Context, uuid.UUID, int) (bool, error)
}

type wagonCommand struct {
	kind, item, anchor string
	situationID        uuid.UUID
	reply              chan error
}

func NewWagonCommand(kind, item, anchor string, situationID uuid.UUID) wagonCommand {
	return wagonCommand{kind: kind, item: item, anchor: anchor, situationID: situationID}
}

type WagonManager struct {
	store    wagonStore
	catalog  content.Catalog
	levels   content.Levels
	mu       sync.Mutex
	runtimes map[uuid.UUID]*wagonRuntime
}

func NewWagonManager(store wagonStore, catalog content.Catalog, levels content.Levels) *WagonManager {
	return &WagonManager{store: store, catalog: catalog, levels: levels, runtimes: map[uuid.UUID]*wagonRuntime{}}
}

// levelByID looks up a level by ID. content.Levels has no such helper (it's a
// plain slice), and a method can't be added to it from outside package
// content, so this is a package-local function instead.
func levelByID(levels content.Levels, id string) (content.Level, bool) {
	for _, lvl := range levels {
		if lvl.ID == id {
			return lvl, true
		}
	}
	return content.Level{}, false
}

func (m *WagonManager) Start(sessionID uuid.UUID, cfg content.WagonClassConfig, level content.Level, state domain.WagonState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.runtimes[sessionID]; exists {
		return
	}
	rt := &wagonRuntime{sessionID: sessionID, store: m.store, catalog: m.catalog, cfg: cfg, level: level, state: state,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())), commands: make(chan wagonCommand), attach: make(chan wagonAttach), detach: make(chan *websocket.Conn), stop: make(chan struct{}), done: make(chan struct{})}
	m.runtimes[sessionID] = rt
	go rt.loop()
}

// Recover resumes persisted active shifts after a server restart.
func (m *WagonManager) Recover(ctx context.Context, classes content.WagonClasses) error {
	sessions, err := m.store.ListActiveWagonSessions(ctx)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		if sess.WagonState == nil {
			continue
		}
		cfg, ok := classes[sess.WagonState.ClassID]
		if !ok || cfg.Status == "coming_soon" {
			continue
		}
		level, ok := levelByID(m.levels, sess.WagonState.LevelID)
		if !ok {
			slog.Error("wagon recover: level not found", "session_id", sess.ID, "level_id", sess.WagonState.LevelID)
			continue
		}
		m.Start(sess.ID, cfg, level, *sess.WagonState)
	}
	return nil
}

type wagonAttach struct {
	conn  *websocket.Conn
	reply chan bool
}

func (m *WagonManager) runtime(sessionID uuid.UUID) *wagonRuntime {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runtimes[sessionID]
}

func (m *WagonManager) Attach(sessionID uuid.UUID, conn *websocket.Conn) bool {
	rt := m.runtime(sessionID)
	if rt == nil {
		return false
	}
	reply := make(chan bool, 1)
	select {
	case rt.attach <- wagonAttach{conn, reply}:
	case <-rt.done:
		return false
	}
	select {
	case ok := <-reply:
		return ok
	case <-rt.done:
		return false
	}
}

func (m *WagonManager) Detach(sessionID uuid.UUID, conn *websocket.Conn) {
	rt := m.runtime(sessionID)
	if rt == nil {
		return
	}
	select {
	case rt.detach <- conn:
	case <-rt.done:
	}
}

func (m *WagonManager) Dispatch(ctx context.Context, sessionID uuid.UUID, cmd wagonCommand) error {
	rt := m.runtime(sessionID)
	if rt == nil {
		return repo.ErrNotFound
	}
	cmd.reply = make(chan error, 1)
	select {
	case rt.commands <- cmd:
	case <-rt.done:
		return repo.ErrNotFound
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-cmd.reply:
		return err
	case <-rt.done:
		return repo.ErrNotFound
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *WagonManager) Stop(sessionID uuid.UUID) {
	m.mu.Lock()
	rt := m.runtimes[sessionID]
	delete(m.runtimes, sessionID)
	m.mu.Unlock()
	if rt != nil {
		close(rt.stop)
		<-rt.done
	}
}

func (m *WagonManager) StopAll() {
	m.mu.Lock()
	ids := make([]uuid.UUID, 0, len(m.runtimes))
	for id := range m.runtimes {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Stop(id)
	}
}

type wagonRuntime struct {
	sessionID uuid.UUID
	store     wagonStore
	catalog   content.Catalog
	cfg       content.WagonClassConfig
	level     content.Level
	state     domain.WagonState
	rng       *rand.Rand
	conn      *websocket.Conn
	commands  chan wagonCommand
	attach    chan wagonAttach
	detach    chan *websocket.Conn
	stop      chan struct{}
	done      chan struct{}
	lastSpawn time.Time
}

func (rt *wagonRuntime) loop() {
	defer close(rt.done)
	interval := time.Duration(rt.cfg.TickS) * time.Second
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-rt.stop:
			if rt.conn != nil {
				_ = rt.conn.Close()
			}
			return
		case a := <-rt.attach:
			if rt.conn != nil && rt.conn != a.conn {
				_ = rt.conn.Close()
			}
			rt.conn = a.conn
			rt.push(time.Now())
			a.reply <- rt.conn != nil
		case conn := <-rt.detach:
			if rt.conn == conn {
				rt.conn = nil
			}
		case cmd := <-rt.commands:
			err := rt.handle(time.Now(), cmd)
			if err != nil {
				rt.write(map[string]string{"type": "error", "message": err.Error()})
			} else {
				rt.push(time.Now())
			}
			cmd.reply <- err
		case now := <-ticker.C:
			rt.tick(now)
		}
	}
}

func (rt *wagonRuntime) handle(now time.Time, cmd wagonCommand) error {
	rt.advance(now)
	var next domain.WagonState
	var err error
	switch cmd.kind {
	case "move_to":
		next, err = ApplyWagonMove(rt.state, cmd.anchor, rt.cfg, now)
	case "pick_item":
		allowed := false
		for _, s := range rt.catalog.Scenarios {
			if s.PhysicalRequirement != nil && s.PhysicalRequirement.Kind == "deliver_item" && s.PhysicalRequirement.Item == cmd.item {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrInvalidWagonAction
		}
		next, err = ApplyPickItem(rt.state, rt.cfg.ServicePointAnchor, cmd.item)
	case "give_item", "redirect":
		sit, lookupErr := rt.store.GetSituation(context.Background(), cmd.situationID)
		if lookupErr != nil {
			return lookupErr
		}
		if sit.SessionID != rt.sessionID || sit.Status != domain.SituationStatusActive || sit.PhysicalActionDone || (sit.TimerDeadline != nil && !now.Before(*sit.TimerDeadline)) {
			return repo.ErrConflict
		}
		var req content.PhysicalRequirement
		if len(sit.PhysicalRequirement) == 0 || json.Unmarshal(sit.PhysicalRequirement, &req) != nil {
			return ErrInvalidWagonAction
		}
		if cmd.kind == "give_item" {
			if req.Kind != "deliver_item" || req.Item != cmd.item {
				return ErrInvalidWagonAction
			}
			next, err = ApplyGiveItem(rt.state, cmd.situationID, cmd.item)
		} else {
			if req.Kind != "redirect" {
				return ErrInvalidWagonAction
			}
			next, err = ApplyRedirect(rt.state, cmd.situationID)
		}
		if err != nil {
			return err
		}
		if err = rt.store.CompleteWagonPhysicalAction(context.Background(), rt.sessionID, cmd.situationID, next); err != nil {
			return err
		}
		rt.state = next
		return nil
	default:
		return ErrInvalidWagonAction
	}
	if err != nil {
		return err
	}
	if err := rt.store.UpdateWagonState(context.Background(), rt.sessionID, next); err != nil {
		return err
	}
	rt.state = next
	return nil
}

func (rt *wagonRuntime) tick(now time.Time) {
	sess, err := rt.store.GetSession(context.Background(), rt.sessionID)
	if err != nil || sess.Status != domain.SessionStatusActive {
		return
	}
	// Closed cases vacate their seats, even if they were closed by the legacy
	// REST endpoint or its timeout worker while no WebSocket was connected.
	situations, err := rt.store.ListSituationsBySession(context.Background(), rt.sessionID)
	if err != nil {
		slog.Error("wagon list situations", "error", err)
		return
	}
	active := map[uuid.UUID]bool{}
	for _, sit := range situations {
		if sit.Status == domain.SituationStatusActive {
			active[sit.ID] = true
		}
	}
	for i, seat := range rt.state.Seats {
		if seat.SituationID != nil && !active[*seat.SituationID] {
			rt.state.Seats[i].SituationID = nil
			rt.state.Seats[i].SituationDefID = nil
			rt.state.Seats[i].Actor = domain.WagonActor{At: seat.Anchor}
			rt.state.Seats[i].RestrictedReached = false
		}
	}
	rt.advance(now)
	interval := time.Duration(rt.cfg.SpawnCheckIntervalS) * time.Second
	if interval <= 0 {
		interval = 3 * time.Second
	}
	if rt.lastSpawn.IsZero() || now.Sub(rt.lastSpawn) >= interval {
		rt.lastSpawn = now
		elapsed := now.Sub(rt.state.StartedAt)
		for _, decision := range PickWagonSpawns(rt.state, rt.cfg, rt.level, rt.catalog.Scenarios, elapsed, rt.rng) {
			rt.spawn(now, decision)
		}
	}
	if err := rt.store.UpdateWagonState(context.Background(), rt.sessionID, rt.state); err != nil {
		slog.Error("wagon persist", "error", err)
		return
	}
	rt.push(now)
}

func (rt *wagonRuntime) advance(now time.Time) {
	state, arrivals := AdvanceWagonMovements(rt.state, now)
	rt.state = state
	for _, a := range arrivals {
		if a.ReachedRestricted && rt.state.Seats[a.SeatIndex].SituationID != nil {
			if err := rt.store.RecordRestrictedArrival(context.Background(), *rt.state.Seats[a.SeatIndex].SituationID); err != nil {
				slog.Error("wagon restricted arrival", "error", err)
			}
		}
	}
}

func (rt *wagonRuntime) spawn(now time.Time, decision WagonSpawnDecision) {
	scenario, ok := findWagonScenario(rt.catalog, decision.ScenarioID)
	if !ok {
		return
	}
	seat := rt.state.Seats[decision.SeatIndex]
	passenger, ok := findWagonPassenger(rt.catalog, seat.PassengerDefID)
	if !ok {
		return
	}
	draft := buildSituationDraft(scenario, passenger)
	draft.SessionID = rt.sessionID
	draft.SeatAnchor = &seat.Anchor
	if scenario.PhysicalRequirement != nil {
		draft.PhysicalRequirement, _ = json.Marshal(scenario.PhysicalRequirement)
	}
	created, err := rt.store.CreateSituation(context.Background(), draft)
	if err != nil {
		slog.Error("wagon spawn", "error", err)
		return
	}
	rt.state.Seats[decision.SeatIndex].SituationID = &created.ID
	rt.state.Seats[decision.SeatIndex].SituationDefID = created.SituationDefID
	if scenario.PhysicalRequirement != nil && scenario.PhysicalRequirement.Kind == "redirect" && len(rt.state.RestrictedAnchors) > 0 {
		rt.state.Seats[decision.SeatIndex].Actor.Moving = &domain.WagonMove{From: seat.Anchor, To: rt.state.RestrictedAnchors[0], StartedAt: now, DurationS: rt.cfg.RedirectDurationS}
	}
}

func findWagonScenario(c content.Catalog, id string) (content.Scenario, bool) {
	for _, s := range c.Scenarios {
		if s.ID == id {
			return s, true
		}
	}
	return content.Scenario{}, false
}
func findWagonPassenger(c content.Catalog, id string) (content.Passenger, bool) {
	for _, p := range c.Passengers {
		if p.ID == id {
			return p, true
		}
	}
	return content.Passenger{}, false
}

func (rt *wagonRuntime) push(now time.Time) {
	type ref struct {
		SituationID uuid.UUID `json:"situation_id"`
		SeatAnchor  string    `json:"seat_anchor"`
		Type        string    `json:"type"`
		Pool        string    `json:"pool"`
	}
	active := make([]ref, 0)
	for _, seat := range rt.state.Seats {
		if seat.SituationID == nil {
			continue
		}
		item := ref{SituationID: *seat.SituationID, SeatAnchor: seat.Anchor}
		if seat.SituationDefID != nil {
			item.Type = *seat.SituationDefID
			if sc, ok := findWagonScenario(rt.catalog, item.Type); ok {
				item.Pool = wagonPool(sc.Criticality)
			}
		}
		active = append(active, item)
	}
	gameTime := int(now.Sub(rt.state.StartedAt).Seconds())
	if gameTime < 0 {
		gameTime = 0
	}
	rt.write(map[string]any{"type": "state", "game_time_s": gameTime, "wagon_state": rt.state, "active_situations": active})
}

func (rt *wagonRuntime) write(v any) {
	if rt.conn == nil {
		return
	}
	_ = rt.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if err := rt.conn.WriteJSON(v); err != nil {
		_ = rt.conn.Close()
		rt.conn = nil
	}
}
