package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ScoringPrompt(input ScoringInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Ты оцениваешь работу проводника ВСМ по регламенту. Отвечай только на фактические вопросы, не ставь баллы и не решай исход.\n\n")
	fmt.Fprintf(&b, "Ситуация: %s\nТип: %s, критичность: %s\nСтартовое описание: %s\nПассажир: %s\n",
		input.Scenario.Title, input.Scenario.Type, input.Scenario.Criticality, input.Scenario.Opening, input.Passenger.PromptHint)
	b.WriteString("\nПроводник должен был донести:\n")
	for _, p := range input.Scenario.CorrectCompletion.MustConvey {
		fmt.Fprintf(&b, "%s: %s\n", p.ID, p.Desc)
	}
	fmt.Fprintf(&b, "\nТребовалась эскалация: %t. Допустимые адресаты: %s.\n", input.Scenario.CorrectCompletion.Escalation.Required, strings.Join(input.Scenario.CorrectCompletion.Escalation.To, ", "))
	b.WriteString("\nПолный диалог:\n")
	for _, m := range input.History {
		fmt.Fprintf(&b, "%s: %s\n", m.Role, m.Content)
	}
	fmt.Fprintf(&b, "\nСписок вызванных адресатов: %s\n", strings.Join(input.Escalations, ", "))
	b.WriteString(`
Верни только JSON по схеме:
{"conveyed":["A"],"missed":["B"],"tone":"neutral","escalation_done":["medic"],"escalation_ok":true,"reasoning":"..."}
conveyed определяй по смыслу, tone — доминирующий тон всех реплик проводника (empathic, neutral или rude).
`)
	return b.String()
}

// ParseScoreResult tolerates a Markdown code fence or a short wrapper. The
// service validates point IDs and derives missing points from conveyed IDs.
func ParseScoreResult(raw string) (ScoreResult, error) {
	start, end := strings.IndexByte(raw, '{'), strings.LastIndexByte(raw, '}')
	if start < 0 || end <= start {
		return ScoreResult{}, fmt.Errorf("scoring response has no JSON object")
	}
	var result ScoreResult
	if err := json.Unmarshal([]byte(raw[start:end+1]), &result); err != nil {
		return ScoreResult{}, fmt.Errorf("scoring response: %w", err)
	}
	return result, nil
}
