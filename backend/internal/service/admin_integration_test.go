package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
	"github.com/mostransport/vsm-trainer/internal/repo/postgres"
)

// newTestAdmin builds an AdminService against the real embedded content for
// integration tests, with the "demo" points namespace matching the server's
// default.
func newTestAdmin(t *testing.T, store *postgres.Store) *AdminService {
	t.Helper()
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	levels, err := content.LoadLevels()
	if err != nil {
		t.Fatal(err)
	}
	curriculum, err := content.LoadCurriculum()
	if err != nil {
		t.Fatal(err)
	}
	return NewAdminService(store, catalog, levels, curriculum, NewPrizeService(store), "demo")
}

// TestLearningSummaryCountsFinishedSessionsOnly covers the outcome
// aggregate's only remaining gate now that the draft/approved ceremony is
// gone: a session counts iff it is finished, full stop. An active session's
// evidence must not leak into the summary; once finished, it must.
func TestLearningSummaryCountsFinishedSessionsOnly(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	player, err := store.CreatePlayer(ctx, fmt.Sprintf("summary-%s@example.invalid", uuid.NewString()), "summary-test", "unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), url)
		if err == nil {
			_, _ = conn.Exec(context.Background(), `DELETE FROM players WHERE id = $1`, player.ID)
			_ = conn.Close(context.Background())
		}
	})
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	scenario := catalog.Scenarios[0]
	scenarioID := scenario.ID
	draft := domain.Situation{Status: domain.SituationStatusActive, SituationDefID: &scenarioID,
		PassengerParams: map[string]any{"opening": scenario.Opening}, Loyalty: 60, Safety: 70}
	sess, situations, err := store.CreateSessionWithSituations(ctx, player.ID, nil, []domain.Situation{draft})
	if err != nil {
		t.Fatal(err)
	}
	outcome := "success"
	sit := situations[0]
	sit.Outcome, sit.XP = &outcome, 25
	if closed, err := store.CloseSituation(ctx, sit); err != nil || !closed {
		t.Fatalf("close: %v, %v", closed, err)
	}

	admin := newTestAdmin(t, store)
	activeSummary, err := admin.LearningSummary(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if activeSummary.SessionOutcomes.CompletedCount != 0 {
		t.Fatalf("active session counted as completed: %+v", activeSummary.SessionOutcomes)
	}
	for _, competency := range activeSummary.Competencies {
		if competency.EvidenceCount != 0 || competency.Score != nil {
			t.Fatalf("active-session evidence leaked: %+v", competency)
		}
	}

	awards := map[string]repo.CompetencyAward{string(scenario.Type): {XP: 25, Evidence: 1}}
	if awarded, err := store.FinishSessionAndAwardXP(ctx, sess.ID, player.ID, 25, awards, 1, 0, time.Now()); err != nil || !awarded {
		t.Fatalf("finish: %v, %v", awarded, err)
	}

	finished, err := admin.LearningSummary(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.SessionOutcomes.CompletedCount != 1 || finished.SessionOutcomes.PassedCount != 1 {
		t.Fatalf("finished summary: %+v", finished.SessionOutcomes)
	}
	found := false
	for _, competency := range finished.Competencies {
		if competency.Code == string(scenario.Type) {
			found = competency.Score != nil && *competency.Score == 25 && competency.EvidenceCount == 1
		}
	}
	if !found {
		t.Fatalf("finished competency missing: %+v", finished.Competencies)
	}
	if finished.TotalXP != 25 {
		t.Fatalf("total_xp = %d, want 25", finished.TotalXP)
	}
}

// TestCreatePlayerAccountWithBrigade covers the admin-only account-creation
// path that replaces public self-registration: the created player carries
// the given brigade name and can log in with the given password.
func TestCreatePlayerAccountWithBrigade(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	admin := newTestAdmin(t, store)
	auth := NewAuthService(store, "test-secret", time.Hour, time.Hour)

	email := fmt.Sprintf("brigade-%s@example.invalid", uuid.NewString())
	const password = "example-secret-1234"
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), url)
		if err == nil {
			_, _ = conn.Exec(context.Background(), `DELETE FROM players WHERE email = $1`, email)
			_ = conn.Close(context.Background())
		}
	})

	player, err := admin.CreatePlayerAccount(ctx, CreatePlayerAccountInput{
		Email:       email,
		Username:    "brigade-test",
		Password:    password,
		BrigadeName: "Depot 7 Brigade",
	})
	if err != nil {
		t.Fatal(err)
	}
	if player.BrigadeID == nil || *player.BrigadeID != "Depot 7 Brigade" {
		t.Fatalf("brigade id = %+v, want %q", player.BrigadeID, "Depot 7 Brigade")
	}

	login, err := auth.Login(ctx, email, password)
	if err != nil {
		t.Fatalf("login with admin-created account: %v", err)
	}
	if login.Player.Email != email || login.Player.BrigadeID == nil || *login.Player.BrigadeID != "Depot 7 Brigade" {
		t.Fatalf("login player: %+v", login.Player)
	}
}

// TestCreatePlayerAccountDuplicateEmail covers the unique-email conflict
// path: a second account with the same email is rejected with ErrEmailTaken,
// not a raw database error.
func TestCreatePlayerAccountDuplicateEmail(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	admin := newTestAdmin(t, store)

	email := fmt.Sprintf("dup-%s@example.invalid", uuid.NewString())
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), url)
		if err == nil {
			_, _ = conn.Exec(context.Background(), `DELETE FROM players WHERE email = $1`, email)
			_ = conn.Close(context.Background())
		}
	})

	in := CreatePlayerAccountInput{Email: email, Username: "dup-test", Password: "example-secret-1234", BrigadeName: "Brigade A"}
	if _, err := admin.CreatePlayerAccount(ctx, in); err != nil {
		t.Fatal(err)
	}
	in.Username = "dup-test-2"
	if _, err := admin.CreatePlayerAccount(ctx, in); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate email error = %v, want ErrEmailTaken", err)
	}
}

// TestCreatePlayerAccountValidation covers the input-validation path: a
// missing brigade name or a too-short password is rejected up front, never
// reaching the store (and never silently defaulting the brigade to empty).
func TestCreatePlayerAccountValidation(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	admin := newTestAdmin(t, store)

	missingBrigade := CreatePlayerAccountInput{
		Email:    fmt.Sprintf("nobrigade-%s@example.invalid", uuid.NewString()),
		Username: "nobrigade-test",
		Password: "example-secret-1234",
	}
	if _, err := admin.CreatePlayerAccount(ctx, missingBrigade); err == nil {
		t.Fatal("missing brigade_name was accepted")
	}

	shortPassword := CreatePlayerAccountInput{
		Email:       fmt.Sprintf("shortpw-%s@example.invalid", uuid.NewString()),
		Username:    "shortpw-test",
		Password:    "abc",
		BrigadeName: "Brigade B",
	}
	if _, err := admin.CreatePlayerAccount(ctx, shortPassword); err == nil {
		t.Fatal("short password was accepted")
	}
}

// TestLearningSummaryWagonProgressionNeverPlayed checks the never-attempted
// baseline: every level shows Attempts: 0, and unlock status follows
// WagonProgress alone (first level unlocked, the rest locked). It also
// checks the same never-touched baseline for LessonProgression: every
// curriculum lesson appears, all Completed: false with zero XP.
//
// TODO: a follow-up wagon-specific integration test should also cover a
// real finished wagon session (attempts count, Passed, LastAttemptAt, and
// the resulting "passed"/"unlocked" transition) once there's harness
// support here for driving a wagon session to completion; that machinery
// (WagonManager, situation resolution loop) isn't part of this file's
// existing fixtures.
func TestLearningSummaryWagonProgressionNeverPlayed(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	player, err := store.CreatePlayer(ctx, fmt.Sprintf("wagon-summary-%s@example.invalid", uuid.NewString()), "wagon-summary-test", "unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), url)
		if err == nil {
			_, _ = conn.Exec(context.Background(), `DELETE FROM players WHERE id = $1`, player.ID)
			_ = conn.Close(context.Background())
		}
	})
	levels, err := content.LoadLevels()
	if err != nil {
		t.Fatal(err)
	}
	curriculum, err := content.LoadCurriculum()
	if err != nil {
		t.Fatal(err)
	}
	admin := newTestAdmin(t, store)
	summary, err := admin.LearningSummary(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.WagonProgression.CurrentProgress != 0 {
		t.Fatalf("current progress = %d, want 0", summary.WagonProgression.CurrentProgress)
	}
	if len(summary.WagonProgression.Levels) != len(levels) {
		t.Fatalf("wagon progression levels = %d, want %d", len(summary.WagonProgression.Levels), len(levels))
	}
	for i, lvl := range summary.WagonProgression.Levels {
		if lvl.Attempts != 0 || lvl.Passed || lvl.LastAttemptAt != nil {
			t.Fatalf("level[%d] should be untouched: %+v", i, lvl)
		}
		wantStatus := "locked"
		if lvl.Order == 1 {
			wantStatus = "unlocked"
		}
		if lvl.Status != wantStatus {
			t.Fatalf("level[%d] (order %d) status = %q, want %q", i, lvl.Order, lvl.Status, wantStatus)
		}
	}
	if len(summary.LessonProgression.Lessons) != len(curriculum.Lessons) {
		t.Fatalf("lesson progression lessons = %d, want %d", len(summary.LessonProgression.Lessons), len(curriculum.Lessons))
	}
	for i, l := range summary.LessonProgression.Lessons {
		if l.Completed || l.TheoryPass || l.PracticePass || l.XPEarned != 0 || l.BadgeID != "" || l.CompletedAt != nil {
			t.Fatalf("lesson[%d] should be untouched: %+v", i, l)
		}
	}
	if summary.TotalXP != 0 || summary.PlayerLevel != levelForXP(0) {
		t.Fatalf("total_xp/player_level for a fresh player = %d/%d", summary.TotalXP, summary.PlayerLevel)
	}
	if len(summary.Achievements) != 0 {
		t.Fatalf("fresh player should have no achievements: %+v", summary.Achievements)
	}
}

// TestLearningSummaryLessonProgressionAndAchievements drives a real lesson
// completion (lesson_progress + lesson_awards rows, via the same store
// methods LearningService.FinalizePractice uses) plus a directly-seeded
// achievement row, and checks that LearningSummary surfaces all of it:
// the completed lesson's XP/badge/Completed flag, the untouched lesson
// still listed with zero XP, the achievement code, and the resulting
// total_xp/player_level.
func TestLearningSummaryLessonProgressionAndAchievements(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	player, err := store.CreatePlayer(ctx, fmt.Sprintf("lesson-summary-%s@example.invalid", uuid.NewString()), "lesson-summary-test", "unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), url)
		if err == nil {
			_, _ = conn.Exec(context.Background(), `DELETE FROM players WHERE id = $1`, player.ID)
			_ = conn.Close(context.Background())
		}
	})

	curriculum, err := content.LoadCurriculum()
	if err != nil {
		t.Fatal(err)
	}
	if len(curriculum.Lessons) < 2 {
		t.Fatal("expected at least 2 curriculum lessons for this test")
	}
	completedLesson := curriculum.Lessons[0]
	untouchedLesson := curriculum.Lessons[1]

	completedAt := time.Now()
	progress := domain.LessonProgress{
		PlayerID:          player.ID,
		LessonID:          completedLesson.LessonID,
		TheoryPass:        true,
		PracticePass:      true,
		PracticeCheckPass: true,
		CompletedAt:       &completedAt,
	}
	if err := store.UpsertLessonProgress(ctx, progress); err != nil {
		t.Fatal(err)
	}
	const lessonXP = 50
	awarded, err := store.AwardLessonCompletion(ctx, player.ID, completedLesson.LessonID, lessonXP, completedLesson.BadgeID)
	if err != nil || !awarded {
		t.Fatalf("award lesson completion: %v, %v", awarded, err)
	}

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, `INSERT INTO achievements (player_id, code) VALUES ($1, 'test_achievement')`, player.ID); err != nil {
		t.Fatal(err)
	}

	admin := newTestAdmin(t, store)
	summary, err := admin.LearningSummary(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}

	var completedEntry, untouchedEntry *LessonProgressionEntry
	for i := range summary.LessonProgression.Lessons {
		entry := &summary.LessonProgression.Lessons[i]
		switch entry.LessonID {
		case completedLesson.LessonID:
			completedEntry = entry
		case untouchedLesson.LessonID:
			untouchedEntry = entry
		}
	}
	if completedEntry == nil || untouchedEntry == nil {
		t.Fatalf("missing lesson entries in %+v", summary.LessonProgression)
	}
	if !completedEntry.Completed || !completedEntry.TheoryPass || !completedEntry.PracticePass {
		t.Fatalf("completed lesson entry: %+v", completedEntry)
	}
	if completedEntry.XPEarned != lessonXP || completedEntry.BadgeID != completedLesson.BadgeID {
		t.Fatalf("completed lesson xp/badge: %+v, want xp=%d badge=%q", completedEntry, lessonXP, completedLesson.BadgeID)
	}
	if completedEntry.CompletedAt == nil {
		t.Fatalf("completed lesson missing completed_at: %+v", completedEntry)
	}
	if untouchedEntry.Completed || untouchedEntry.XPEarned != 0 || untouchedEntry.BadgeID != "" {
		t.Fatalf("untouched lesson entry should be blank: %+v", untouchedEntry)
	}

	if summary.TotalXP != lessonXP {
		t.Fatalf("total_xp = %d, want %d", summary.TotalXP, lessonXP)
	}
	if summary.PlayerLevel != levelForXP(lessonXP) {
		t.Fatalf("player_level = %d, want %d", summary.PlayerLevel, levelForXP(lessonXP))
	}
	found := false
	for _, code := range summary.Achievements {
		if code == "test_achievement" {
			found = true
		}
	}
	if !found {
		t.Fatalf("achievements missing seeded code: %+v", summary.Achievements)
	}
}
