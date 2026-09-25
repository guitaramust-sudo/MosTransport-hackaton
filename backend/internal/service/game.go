package service

import "fmt"

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func buildSystemPrompt(promptHint, language, traits, opening string) string {
	return fmt.Sprintf(
		"Ты — пассажир в тренажёре проводников ВСМ. Отвечай только от лица пассажира, "+
			"коротко и естественно (1–3 предложения), без пояснений и мета-комментариев.\n\n"+
			"Пассажир: %s. Язык: %s. Черты: %s.\n\nСитуация: %s",
		promptHint, language, traits, opening,
	)
}
