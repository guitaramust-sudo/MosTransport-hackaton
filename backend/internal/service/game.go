package service

import (
	"fmt"
	"strings"
)

// CategoryEffect describes how a classified player reply moves the two scales
// and which competency it trains. This is the explainable core of the game.
type CategoryEffect struct {
	LoyaltyDelta int
	SafetyDelta  int
	Competency   string // competency code that gains XP; empty = none
	XP           int
}

// Categories is the closed set used for classification.
var Categories = []string{"эмпатия", "давление", "отстранение", "вызов помощи", "неверное действие"}

// categoryEffects maps a classification to its effect on the two scales.
var categoryEffects = map[string]CategoryEffect{
	"эмпатия":           {LoyaltyDelta: 8, SafetyDelta: 1, Competency: "communication", XP: 10},
	"давление":          {LoyaltyDelta: -10, SafetyDelta: 2, Competency: "safety", XP: 6},
	"отстранение":       {LoyaltyDelta: -8, SafetyDelta: -4, Competency: "", XP: 2},
	"вызов помощи":      {LoyaltyDelta: -2, SafetyDelta: 10, Competency: "safety", XP: 12},
	"неверное действие": {LoyaltyDelta: -8, SafetyDelta: -12, Competency: "", XP: 1},
}

func effectFor(category string) CategoryEffect {
	if e, ok := categoryEffects[category]; ok {
		return e
	}
	return CategoryEffect{}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// PassengerArchetype is a procedural situation template.
type PassengerArchetype struct {
	Code     string
	Name     string
	Persona  string // LLM role-play instructions about the passenger
	Scenario string // what the conductor sees
	Opening  string // passenger's first line
}

// archetypes is a small hard-coded pool; the session generator picks from it.
var archetypes = []PassengerArchetype{
	{
		Code:     "no_ticket",
		Name:     "Сергей",
		Persona:  "Сергей, 35 лет, усталый командировочный. Уверен, что билет был, но не может его найти. Сначала нервничает, потом раздражается, если на него давить.",
		Scenario: "Пассажир утверждает, что купил билет, но не может его найти. Контроль билетов уже рядом.",
		Opening:  "Молодой человек, я точно брал билет! Вот здесь был… Куда он подевался?",
	},
	{
		Code:     "heavy_luggage",
		Name:     "Анна",
		Persona:  "Анна, 60 лет, едет к внукам с тяжёлым чемоданом. Сама поднять на полку не может. Обижается на грубость, благодарна за помощь.",
		Scenario: "Пожилая пассажирка не может поднять тяжёлый чемодан на багажную полку.",
		Opening:  "Простите, не поможете? Чемодан такой тяжёлый, я его не подниму…",
	},
	{
		Code:     "loud_music",
		Name:     "Дмитрий",
		Persona:  "Дмитрий, 22 года, слушает музыку в наушниках, но так громко, что слышно всем. Не сразу понимает, что мешает. Реагирует на вежливую просьбу.",
		Scenario: "Пассажир слушает музыку настолько громко, что мешает другим пассажирам в вагоне.",
		Opening:  "А? Вы что-то сказали? (продолжает кивать в такт музыке)",
	},
	{
		Code:     "lost_item",
		Name:     "Ольга",
		Persona:  "Ольга, 45 лет, в панике: потеряла телефон с билетом и картами. Эмоциональна, ей нужна поддержка и чёткие действия.",
		Scenario: "Пассажирка в панике — потеряла телефон, где были билет и банковские карты.",
		Opening:  "Помогите! Я потеряла телефон, там всё — билет, карты, документы! Что мне делать?!",
	},
	{
		Code:     "unwell",
		Name:     "Виктор",
		Persona:  "Виктор, 70 лет, почувствовал себя плохо: кружится голова, слабость. Стесняется просить о помощи, но состояние ухудшается.",
		Scenario: "Пожилому пассажиру стало плохо — ему нужна медицинская помощь.",
		Opening:  "Что-то мне нехорошо… Голова кружится. Не знаю, наверное, пройдёт…",
	},
	{
		Code:     "aggressive",
		Name:     "Пётр",
		Persona:  "Пётр, 40 лет, выпил, ведёт себя агрессивно, громко возмущается и провоцирует. Требует особого отношения. Опасен, но разговаривать можно.",
		Scenario: "Агрессивный пассажир в состоянии опьянения громко возмущается и мешает другим пассажирам.",
		Opening:  "Эй, проводник! А почему у вас тут так тесно?! Это безобразие! Я жаловаться буду!",
	},
}

func buildSystemPrompt(name, persona, scenario string) string {
	return fmt.Sprintf(
		"Ты участвуешь в игре-тренажёре для обучения проводников ВСМ. Ты играешь роль пассажира. "+
			"Отвечай ТОЛЬКО от лица пассажира, коротко (1-3 предложения), естественно, без описаний действий "+
			"от третьего лица и без каких-либо пояснений или мета-комментариев.\n\n"+
			"Пассажир: %s. %s\n\nСитуация: %s",
		name, persona, scenario,
	)
}

// situationXP is the experience earned for resolving a situation, scaled by
// how well both loyalty and safety ended up.
func situationXP(loyalty, safety int) int {
	return 20 + (loyalty+safety)/10
}

// outcomeLabel derives a human-readable outcome from the final scales.
func outcomeLabel(loyalty, safety int) string {
	avg := (loyalty + safety) / 2
	switch {
	case avg >= 70:
		return "resolved_positive"
	case avg >= 40:
		return "resolved_neutral"
	default:
		return "resolved_negative"
	}
}

// normalizeCategory makes classification robust against extra punctuation/case.
func normalizeCategory(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	for _, c := range Categories {
		if strings.EqualFold(s, c) {
			return c
		}
	}
	for _, c := range Categories {
		if strings.Contains(s, c) {
			return c
		}
	}
	return "эмпатия" // safe default
}
