package llm

import (
	"context"
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
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			last = strings.ToLower(messages[i].Content)
			break
		}
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

var _ LLMClient = (*MockLLM)(nil)

func (m *MockLLM) String() string { return fmt.Sprintf("mock-llm") }
