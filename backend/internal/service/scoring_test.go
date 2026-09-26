package service

import (
	"testing"
	"time"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/llm"
)

func testScenario(points int, required bool, targets []string) content.Scenario {
	s := content.Scenario{TimeLimitSec: 60}
	for i := 0; i < points; i++ {
		s.CorrectCompletion.MustConvey = append(s.CorrectCompletion.MustConvey, content.MustConvey{
			ID: string(rune('A' + i)), Desc: "Описание пункта",
		})
	}
	s.CorrectCompletion.Escalation = content.Escalation{Required: required, To: targets}
	return s
}

func TestEvaluateScoreExample(t *testing.T) {
	scenario := testScenario(3, true, []string{content.TargetTrainChief, content.TargetMedic})
	score := llm.ScoreResult{
		Conveyed: []string{"A", "B", "INVALID", "A"}, Missed: []string{"A"},
		Tone: "empathic", EscalationOK: false,
	}
	got := EvaluateScore(scenario, score, []string{content.TargetTrainChief, content.TargetMedic}, 25*time.Second, false)
	if got.Outcome != "partial" || got.XP != 20 || got.Safety != 65 || got.Loyalty != 60 {
		t.Fatalf("example = %+v, want partial/20/65/60 from the remark table", got)
	}
	if len(got.Conveyed) != 2 || len(got.Missed) != 1 || got.Missed[0] != "C" {
		t.Fatalf("point normalization = conveyed %v, missed %v", got.Conveyed, got.Missed)
	}
}

func TestEvaluateScoreOutcomes(t *testing.T) {
	cases := []struct {
		name       string
		points     int
		conveyed   []string
		required   bool
		targets    []string
		actual     []string
		elapsed    time.Duration
		want       string
		remarkCode string
	}{
		{"success", 3, []string{"A", "B", "C"}, false, nil, nil, 40 * time.Second, "success", "solved"},
		{"timeout still scored", 3, []string{"A", "B", "C"}, false, nil, nil, 60 * time.Second, "timeout", "timeout"},
		{"no escalation", 3, []string{"A", "B", "C"}, true, []string{content.TargetMedic}, nil, 10 * time.Second, "fail", "no_escalation"},
		{"one point missed", 1, nil, false, nil, nil, 10 * time.Second, "fail", "missed_point"},
		{"five points two missed", 5, []string{"A", "B", "C"}, false, nil, nil, 10 * time.Second, "partial", "missed_point"},
		{"five points three missed", 5, []string{"A", "B"}, false, nil, nil, 10 * time.Second, "fail", "missed_point"},
		{"false escalation", 1, []string{"A"}, false, nil, []string{content.TargetMedic}, 10 * time.Second, "success", "false_escalation"},
		{"wrong optional target", 1, []string{"A"}, false, []string{content.TargetTrainChief}, []string{content.TargetPolice}, 10 * time.Second, "success", "wrong_target"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateScore(testScenario(tc.points, tc.required, tc.targets), llm.ScoreResult{Conveyed: tc.conveyed}, tc.actual, tc.elapsed, false)
			if got.Outcome != tc.want {
				t.Fatalf("outcome = %s, want %s", got.Outcome, tc.want)
			}
			found := false
			for _, remark := range got.Remarks {
				found = found || remark.Code == tc.remarkCode
			}
			if !found {
				t.Fatalf("remarks %v lack %s", got.Remarks, tc.remarkCode)
			}
		})
	}
}

func TestForcedTimeoutStillScoresFacts(t *testing.T) {
	got := EvaluateScore(testScenario(1, false, nil), llm.ScoreResult{Conveyed: []string{"A"}}, nil, 59*time.Second, true)
	if got.Outcome != "timeout" || len(got.Missed) != 0 {
		t.Fatalf("forced timeout = %+v", got)
	}
}

func TestEscalationSatisfiesLinkedPoint(t *testing.T) {
	scenario := content.Scenario{TimeLimitSec: 60}
	scenario.CorrectCompletion.MustConvey = []content.MustConvey{
		{ID: "A", Desc: "Уточнить, где запах сильнее"},
		{ID: "B", Desc: "Предупредить пассажиров держаться подальше"},
		{ID: "C", Desc: "Сообщить начальнику поезда о запахе", EscalationTarget: content.TargetTrainChief},
	}
	scenario.CorrectCompletion.Escalation = content.Escalation{Required: true, To: []string{content.TargetTrainChief}}

	// The LLM reports everything missed, but the player escalated to
	// train_chief, so the escalation-linked point C must be auto-conveyed.
	got := EvaluateScore(scenario, llm.ScoreResult{Conveyed: []string{}}, []string{content.TargetTrainChief}, 30*time.Second, false)

	if len(got.Conveyed) != 1 || got.Conveyed[0] != "C" {
		t.Fatalf("conveyed = %v, want [C]", got.Conveyed)
	}
	if len(got.Missed) != 2 || contains(got.Missed, "C") {
		t.Fatalf("missed = %v, want [A B]", got.Missed)
	}
	if got.Outcome != "fail" {
		t.Fatalf("outcome = %s, want fail (2 of 3 still missed)", got.Outcome)
	}
}
