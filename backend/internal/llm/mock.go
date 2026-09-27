package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MockLLM is an offline implementation of LLMClient used for development
// and demos. It never touches the network.
type MockLLM struct{}

func NewMockLLM() *MockLLM { return &MockLLM{} }

// Chat produces a canned passenger reply. It looks at the last user message
// to pick a reply flavour so the demo feels responsive.
func (m *MockLLM) Chat(ctx context.Context, messages []Message) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	last := ""
	language := "ru"
	system := ""
	for _, message := range messages {
		if message.Role != "system" {
			continue
		}
		system = message.Content
		for _, candidate := range []string{"en", "zh", "de"} {
			if strings.Contains(message.Content, "Язык: "+candidate+".") {
				language = candidate
			}
		}
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			last = strings.ToLower(messages[i].Content)
			break
		}
	}
	if strings.Contains(system, "VSM_SIM_OPENING_V1") {
		request := "услуга"
		for _, item := range []string{"плед", "стакан воды", "помощь с багажом"} {
			if strings.Contains(system, item) {
				request = item
				break
			}
		}
		return "Здравствуйте. Мне нужен " + request + ". Поможете?", nil
	}
	if strings.Contains(system, "VSM_SIM_DIALOGUE_V1") {
		choice := ""
		match := func(id string, words ...string) bool {
			if !strings.Contains(system, id+":") {
				return false
			}
			for _, word := range words {
				if strings.Contains(last, word) {
					return true
				}
			}
			return false
		}
		switch {
		case match("check_availability", "провер", "уточню наличие"):
			choice = "check_availability"
		case match("promise_immediately", "обещаю", "принесу", "точно будет"):
			choice = "promise_immediately"
		case match("explain_next_step", "сообщаю", "принесу", "следующ"):
			choice = "explain_next_step"
		case match("correct_promise", "извин", "альтернатив"):
			choice = "correct_promise"
		case match("ignore_followup", "не буду", "не моя проблема"):
			choice = "ignore_followup"
		case match("check_tickets", "билет", "провер"):
			choice = "check_tickets"
		case match("dismiss_dispute", "разбирайтесь", "не моя проблема"):
			choice = "dismiss_dispute"
		case match("report_spill", "предупреж", "устран", "уберу"):
			choice = "report_spill"
		case match("walk_past", "пройду мимо", "ничего делать"):
			choice = "walk_past"
		case match("explain_closed_window", "объясню", "предложу"):
			choice = "explain_closed_window"
		}
		reply := "Пожалуйста, расскажите, что вы собираетесь сделать."
		if choice != "" {
			reply = "Спасибо, я понял ваш следующий шаг."
		}
		encoded, _ := json.Marshal(struct {
			Reply    string `json:"reply"`
			ChoiceID string `json:"choice_id"`
		}{reply, choice})
		return string(encoded), nil
	}
	// Keep the offline demo usable for passengers who do not speak Russian.
	switch language {
	case "en":
		return "I understand. Could you tell me what happens next?", nil
	case "zh":
		return "我明白了。请告诉我接下来该怎么办？", nil
	case "de":
		return "Ich verstehe. Können Sie mir sagen, was als Nächstes passiert?", nil
	}

	switch {
	case strings.Contains(last, "билет"):
		return "У меня есть билет, я его точно брал! Проверьте ещё раз, пожалуйста.", nil
	case strings.Contains(last, "помощь") || strings.Contains(last, "врач") || strings.Contains(last, "плохо"):
		return "Да, мне нехорошо… Я бы хотел, чтобы кто-то подошёл и помог.", nil
	case strings.Contains(last, "багаж") || strings.Contains(last, "полк"):
		return "Эта сумка очень тяжёлая, я сам не справлюсь. Можете помочь поднять?", nil
	case strings.Contains(last, "тише") || strings.Contains(last, "музык") || strings.Contains(last, "шум"):
		return "Извините, я просто слушаю музыку. Сейчас сделаю тише.", nil
	case strings.Contains(last, "выйти") || strings.Contains(last, "остановк") || strings.Contains(last, "станц"):
		return "Я думал, эта станция моя. Что мне теперь делать?", nil
	default:
		return "Понимаю… И что вы предлагаете сделать в такой ситуации?", nil
	}
}

// Classify uses simple keyword heuristics to map text to a category.
func (m *MockLLM) Classify(ctx context.Context, text string, categories []string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	t := strings.ToLower(text)
	score := map[string]int{}
	for _, c := range categories {
		score[c] = 0
	}

	keywords := map[string][]string{
		"эмпатия":           {"понимаю", "сочувствую", "извините", "простите", "пожалуйста", "помогу", "не переживайте", "всё будет хорошо"},
		"давление":          {"немедленно", "обязан", "требую", "запрещено", "иначе", "вызову", "должны", "сейчас же", "это ваша обязанность"},
		"отстранение":       {"не моя проблема", "разберётесь", "мне всё равно", "сами виноваты", "отойдите", "не мешайте", "я занят"},
		"вызов помощи":      {"помощь", "врач", "вызову", "бригад", "медик", "наряд", "подмог", "вызову бригаду", "нужна помощь"},
		"неверное действие": {"не знаю", "плевать", "груб", "оскорб", "удар", "толкну", "выкину", "вон"},
	}

	for _, c := range categories {
		for _, kw := range keywords[c] {
			if strings.Contains(t, kw) {
				score[c]++
			}
		}
	}

	best := categories[0]
	bestScore := score[best]
	for _, c := range categories {
		if score[c] > bestScore {
			best = c
			bestScore = score[c]
		}
	}

	if bestScore == 0 {
		return "эмпатия", nil
	}
	return best, nil
}

func (m *MockLLM) ScoreDialogue(ctx context.Context, input ScoringInput) (ScoreResult, error) {
	if err := ctx.Err(); err != nil {
		return ScoreResult{}, err
	}
	var playerText strings.Builder
	for _, turn := range input.History {
		if turn.Role == "user" {
			playerText.WriteString(" ")
			playerText.WriteString(strings.ToLower(turn.Content))
		}
	}
	text := playerText.String()
	result := ScoreResult{Tone: "neutral", EscalationDone: input.Escalations, Reasoning: "mock keyword scoring"}
	if containsAny(text, "заткнись", "плевать", "идиот", "грубо") {
		result.Tone = "rude"
	} else if containsAny(text, "пожалуйста", "спасибо", "понимаю", "не переживайте", "помогу") {
		result.Tone = "empathic"
	}
	for _, point := range input.Scenario.CorrectCompletion.MustConvey {
		if matchesDescription(text, point.Desc) {
			result.Conveyed = append(result.Conveyed, point.ID)
		} else {
			result.Missed = append(result.Missed, point.ID)
		}
	}
	for _, required := range input.Scenario.CorrectCompletion.Escalation.To {
		if !containsString(input.Escalations, required) {
			return result, nil
		}
	}
	result.EscalationOK = true
	return result, nil
}

func containsAny(text string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func matchesDescription(text, desc string) bool {
	for _, word := range strings.Fields(strings.ToLower(desc)) {
		word = strings.Trim(word, ",.;:!?")
		if len([]rune(word)) >= 5 && strings.Contains(text, word) {
			return true
		}
	}
	return false
}

var _ LLMClient = (*MockLLM)(nil)

func (m *MockLLM) String() string { return fmt.Sprintf("mock-llm") }
