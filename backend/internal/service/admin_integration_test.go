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

func TestLearningSummaryUsesApprovedEvidenceOnly(t *testing.T) {
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
	sess, situations, err := store.CreateSessionWithSituations(ctx, player.ID, []string{"unplayed"}, []domain.Situation{draft})
	if err != nil {
		t.Fatal(err)
	}
	outcome := "success"
	sit := situations[0]
	sit.Outcome, sit.XP = &outcome, 25
	if closed, err := store.CloseSituation(ctx, sit); err != nil || !closed {
		t.Fatalf("close: %v, %v", closed, err)
	}
	awards := map[string]repo.CompetencyAward{string(scenario.Type): {XP: 25, Evidence: 1}}
	if awarded, err := store.FinishSessionAndAwardXP(ctx, sess.ID, player.ID, 25, awards, 1, 1, time.Now()); err != nil || !awarded {
		t.Fatalf("finish: %v, %v", awarded, err)
	}
	levels, err := content.LoadLevels()
	if err != nil {
		t.Fatal(err)
	}
	admin := NewAdminService(store, catalog, levels)
	draftSummary, err := admin.LearningSummary(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if draftSummary.DataStatus != "no_approved_data" || draftSummary.SessionOutcomes.ApprovedCompletedCount != 0 {
		t.Fatalf("draft counted as approved: %+v", draftSummary)
	}
	for _, competency := range draftSummary.Competencies {
		if competency.EvidenceCount != 0 || competency.Score != nil {
			t.Fatalf("draft evidence leaked: %+v", competency)
		}
	}
	if err := admin.ApproveSession(ctx, sess.ID); !errors.Is(err, ErrUnapprovedContent) {
		t.Fatalf("draft scenario was approved: %v", err)
	}
	catalog.Scenarios[0].ValidationStatus = "approved"
	catalog.Scenarios[0].ReviewerID = "test-reviewer"
	catalog.Scenarios[0].SourceRefs = []string{"test-source"}
	if err := admin.ApproveSession(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	approved, err := admin.LearningSummary(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if approved.DataStatus != "available" || approved.SessionOutcomes.ApprovedCompletedCount != 1 {
		t.Fatalf("approved summary: %+v", approved)
	}
	if approved.SessionOutcomes.RecentAssessments[0].SessionPass || approved.SessionOutcomes.RecentAssessments[0].UnresolvedCommitments != 1 {
		t.Fatalf("unplayed situation was ignored: %+v", approved.SessionOutcomes.RecentAssessments[0])
	}
	found := false
	for _, competency := range approved.Competencies {
		if competency.Code == string(scenario.Type) {
			found = competency.Score != nil && *competency.Score == 25 && competency.EvidenceCount == 1
		}
	}
	if !found {
		t.Fatalf("approved competency missing: %+v", approved.Competencies)
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
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	levels, err := content.LoadLevels()
	if err != nil {
		t.Fatal(err)
	}
	admin := NewAdminService(store, catalog, levels)
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
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	levels, err := content.LoadLevels()
	if err != nil {
		t.Fatal(err)
	}
	admin := NewAdminService(store, catalog, levels)

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
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	levels, err := content.LoadLevels()
	if err != nil {
		t.Fatal(err)
	}
	admin := NewAdminService(store, catalog, levels)

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
// WagonProgress alone (first level unlocked, the rest locked).
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
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	levels, err := content.LoadLevels()
	if err != nil {
		t.Fatal(err)
	}
	admin := NewAdminService(store, catalog, levels)
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
}
