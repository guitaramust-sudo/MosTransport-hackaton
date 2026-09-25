package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/llm"
)

type Remark struct {
	Code    string `json:"code"`
	XP      int    `json:"xp"`
	Safety  int    `json:"safety"`
	Loyalty int    `json:"loyalty"`
	Message string `json:"message"`
}

type ScoreSummary struct {
	Outcome   string          `json:"outcome"`
	Tone      string          `json:"tone"`
	Conveyed  []string        `json:"conveyed"`
	Missed    []string        `json:"missed"`
	XP        int             `json:"xp"`
	Safety    int             `json:"safety"`
	Loyalty   int             `json:"loyalty"`
	Remarks   []Remark        `json:"remarks"`
	LLMResult llm.ScoreResult `json:"-"`
}

var remarkTable = map[string]Remark{
	"fast":             {"fast", 15, 0, 5, "Быстрое решение"},
	"on_time":          {"on_time", 5, 0, 0, "Уложился в регламентное время"},
	"timeout":          {"timeout", -20, -10, -10, "Не успел закрыть ситуацию"},
	"escalation_ok":    {"escalation_ok", 10, 15, 0, "Эскалация выполнена верно"},
	"no_escalation":    {"no_escalation", -40, -40, 0, "Обязательная эскалация не выполнена"},
	"false_escalation": {"false_escalation", -15, -10, 0, "Эскалация не требовалась"},
	"wrong_target":     {"wrong_target", -10, -5, 0, "Вызван неверный адресат"},
	"empathic":         {"empathic", 5, 0, 10, "Эмпатичное общение"},
	"rude":             {"rude", -30, 0, -25, "Грубое общение с пассажиром"},
	"missed_point":     {"missed_point", -10, 0, -5, "Пропущен пункт"},
	"solved":           {"solved", 20, 10, 15, "Ситуация решена"},
}

// EvaluateScore is pure: the model reports facts, while outcome, remarks,
// XP and final scales are computed here from the scenario and recorded actions.
func EvaluateScore(scenario content.Scenario, observed llm.ScoreResult, actualEscalations []string, elapsed time.Duration, forcedTimeout bool) ScoreSummary {
	result := ScoreSummary{LLMResult: observed, Remarks: []Remark{}, Conveyed: []string{}, Missed: []string{}}
	conveyed := map[string]bool{}
	for _, id := range observed.Conveyed {
		conveyed[strings.TrimSpace(id)] = true
	}
	for _, point := range scenario.CorrectCompletion.MustConvey {
		if conveyed[point.ID] {
			result.Conveyed = append(result.Conveyed, point.ID)
		} else {
			result.Missed = append(result.Missed, point.ID)
		}
	}
	result.Tone = observed.Tone
	if result.Tone != "empathic" && result.Tone != "neutral" && result.Tone != "rude" {
		result.Tone = "neutral"
	}

	limit := time.Duration(scenario.TimeLimitSec) * time.Second
	timedOut := forcedTimeout || elapsed >= limit
	switch {
	case timedOut:
		result.Remarks = append(result.Remarks, remarkTable["timeout"])
	case elapsed < limit/2:
		result.Remarks = append(result.Remarks, remarkTable["fast"])
	default:
		result.Remarks = append(result.Remarks, remarkTable["on_time"])
	}

	escalationOK, escalationRemark := resolveEscalation(scenario.CorrectCompletion.Escalation, actualEscalations)
	if escalationRemark != "" {
		result.Remarks = append(result.Remarks, remarkTable[escalationRemark])
	}
	if result.Tone == "empathic" || result.Tone == "rude" {
		result.Remarks = append(result.Remarks, remarkTable[result.Tone])
	}
	for _, id := range result.Missed {
		for _, point := range scenario.CorrectCompletion.MustConvey {
			if point.ID == id {
				r := remarkTable["missed_point"]
				r.Message = fmt.Sprintf("Пропущен пункт: %s", point.Desc)
				result.Remarks = append(result.Remarks, r)
				break
			}
		}
	}
	switch {
	case timedOut:
		result.Outcome = "timeout"
	case len(result.Missed) == 0 && escalationOK:
		result.Outcome = "success"
	case len(result.Missed) >= (len(scenario.CorrectCompletion.MustConvey)+1)/2 || (scenario.CorrectCompletion.Escalation.Required && !escalationOK):
		result.Outcome = "fail"
	default:
		result.Outcome = "partial"
	}
	if result.Outcome == "success" {
		result.Remarks = append(result.Remarks, remarkTable["solved"])
	}
	safetyDelta, loyaltyDelta := 0, 0
	for _, r := range result.Remarks {
		result.XP += r.XP
		safetyDelta += r.Safety
		loyaltyDelta += r.Loyalty
	}
	result.Safety = clamp(50+safetyDelta, 0, 100)
	result.Loyalty = clamp(50+loyaltyDelta, 0, 100)
	return result
}

func resolveEscalation(required content.Escalation, actual []string) (bool, string) {
	if required.Required {
		for _, target := range required.To {
			if !contains(actual, target) {
				return false, "no_escalation"
			}
		}
		return true, "escalation_ok"
	}
	if len(actual) == 0 {
		return true, ""
	}
	if len(required.To) == 0 {
		return true, "false_escalation"
	}
	for _, target := range actual {
		if !contains(required.To, target) {
			return true, "wrong_target"
		}
	}
	return true, ""
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
