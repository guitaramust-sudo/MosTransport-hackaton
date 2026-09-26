package service

import (
	"context"
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
	admin := NewAdminService(store, catalog)
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
