package content

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed chapters.json lessons.json questions.json
var curriculumFiles embed.FS

// Chapter is one named unit of the sequential learning map (§27-29 of the
// design doc). Order is 1-based; chapters are shown to the player even when
// locked/empty, so LessonIDs may be empty for a not-yet-authored chapter.
type Chapter struct {
	ChapterID        string   `json:"chapter_id"`
	Order            int      `json:"order"`
	Title            string   `json:"title"`
	LessonIDs        []string `json:"lesson_ids"`
	Scope            string   `json:"scope"` // "common" | "class"
	ClassID          string   `json:"class_id,omitempty"`
	ContentVersion   string   `json:"content_version"`
	ValidationStatus string   `json:"validation_status"`
}

// Lesson is one LOCKED→THEORY→THEORY_CHECK→PRACTICE→PRACTICE_CHECK→DEBRIEF→
// COMPLETED unit. Practice is either pure exploration (RequiredAnchorIDs/
// RequiredObjectIDs non-empty, ScenarioID empty — e.g. B01) or a scripted
// wagon scenario (ScenarioID/MandatoryEventIDs set, started via
// WagonManager.StartScripted — e.g. B02). CompletionRule names which check
// a later task's finalize logic runs: "visit_inspect" or "scenario_result".
type Lesson struct {
	LessonID            string   `json:"lesson_id"`
	ChapterID           string   `json:"chapter_id"`
	Order               int      `json:"order"`
	Title               string   `json:"title"`
	BadgeID             string   `json:"badge_id"`
	EstimatedMin        int      `json:"estimated_min"`
	TheoryCards         []string `json:"theory_cards"`
	TheoryQuestionIDs   []string `json:"theory_question_ids"`
	ScenarioID          string   `json:"scenario_id,omitempty"`
	MandatoryEventIDs   []string `json:"mandatory_event_ids,omitempty"`
	RequiredAnchorIDs   []string `json:"required_anchor_ids,omitempty"`
	RequiredObjectIDs   []string `json:"required_object_ids,omitempty"`
	PracticeQuestionIDs []string `json:"practice_question_ids"`
	CompletionRule      string   `json:"completion_rule"`
	DebriefIntro        string   `json:"debrief_intro"`
	TargetCompetencies  []string `json:"target_competencies,omitempty"`
	SourceRefs          []string `json:"source_refs,omitempty"`
	ValidationStatus    string   `json:"validation_status"`
}

// QuestionOption is one answer choice. Display order in Options is the
// canonical order stored in content; a later task (API layer) is
// responsible for any per-attempt shuffling described in the design doc —
// this content model just holds the authored option set.
type QuestionOption struct {
	OptionID string `json:"option_id"`
	Text     string `json:"text"`
}

// Question holds the server-only answer key alongside the player-facing
// prompt/options. A later task's API layer MUST strip CorrectOptionID (and
// arguably FeedbackByAnswer, since it can leak the answer) before this
// reaches an HTTP response — this type itself is the authoritative content
// record, not a wire DTO, and deliberately is not JSON-safe to serve as-is.
type Question struct {
	QuestionID       string            `json:"question_id"`
	Phase            string            `json:"phase"` // "theory" | "practice"
	Type             string            `json:"type"`  // "single_choice" | "dialogue_choice" | "short_text"
	Prompt           string            `json:"prompt"`
	Options          []QuestionOption  `json:"options,omitempty"`
	CorrectOptionID  string            `json:"correct_option_id"`
	FeedbackByAnswer map[string]string `json:"feedback_by_answer,omitempty"`
	ValidationStatus string            `json:"validation_status"`
}

// Curriculum is the whole loaded, cross-validated content set.
type Curriculum struct {
	Chapters  []Chapter
	Lessons   []Lesson
	Questions []Question
}

func LoadCurriculum() (Curriculum, error) {
	chaptersRaw, err := curriculumFiles.ReadFile("chapters.json")
	if err != nil {
		return Curriculum{}, err
	}
	lessonsRaw, err := curriculumFiles.ReadFile("lessons.json")
	if err != nil {
		return Curriculum{}, err
	}
	questionsRaw, err := curriculumFiles.ReadFile("questions.json")
	if err != nil {
		return Curriculum{}, err
	}
	catalog, err := Load()
	if err != nil {
		return Curriculum{}, err
	}
	classes, err := LoadWagonClasses()
	if err != nil {
		return Curriculum{}, err
	}
	return ParseCurriculum(chaptersRaw, lessonsRaw, questionsRaw, catalog, classes)
}

// ParseCurriculum also lets tests validate a candidate curriculum content
// set against a candidate catalog/classes before embedding it in a build.
func ParseCurriculum(chaptersRaw, lessonsRaw, questionsRaw []byte, catalog Catalog, classes WagonClasses) (Curriculum, error) {
	var chapters []Chapter
	if err := json.Unmarshal(chaptersRaw, &chapters); err != nil {
		return Curriculum{}, fmt.Errorf("chapters.json: %w", err)
	}
	var lessons []Lesson
	if err := json.Unmarshal(lessonsRaw, &lessons); err != nil {
		return Curriculum{}, fmt.Errorf("lessons.json: %w", err)
	}
	var questions []Question
	if err := json.Unmarshal(questionsRaw, &questions); err != nil {
		return Curriculum{}, fmt.Errorf("questions.json: %w", err)
	}

	if len(chapters) == 0 {
		return Curriculum{}, fmt.Errorf("chapters.json: must define at least one chapter")
	}

	// --- Questions ---------------------------------------------------
	questionsByID := map[string]Question{}
	for i, q := range questions {
		where := fmt.Sprintf("question[%d]", i)
		if blank(q.QuestionID) || questionsByID[q.QuestionID].QuestionID != "" {
			return Curriculum{}, fmt.Errorf("%s: empty or duplicate question_id %q", where, q.QuestionID)
		}
		if !oneOf(q.Phase, "theory", "practice") {
			return Curriculum{}, fmt.Errorf("%s (id %q): invalid phase %q", where, q.QuestionID, q.Phase)
		}
		if !oneOf(q.Type, "single_choice", "dialogue_choice", "short_text") {
			return Curriculum{}, fmt.Errorf("%s (id %q): invalid type %q", where, q.QuestionID, q.Type)
		}
		if blank(q.Prompt) {
			return Curriculum{}, fmt.Errorf("%s (id %q): prompt is required", where, q.QuestionID)
		}
		if q.Type == "single_choice" || q.Type == "dialogue_choice" {
			if len(q.Options) < 2 {
				return Curriculum{}, fmt.Errorf("%s (id %q): %s requires at least 2 options, got %d", where, q.QuestionID, q.Type, len(q.Options))
			}
			optionIDs := map[string]bool{}
			for _, opt := range q.Options {
				if blank(opt.OptionID) || optionIDs[opt.OptionID] {
					return Curriculum{}, fmt.Errorf("%s (id %q): empty or duplicate option_id %q", where, q.QuestionID, opt.OptionID)
				}
				optionIDs[opt.OptionID] = true
			}
			if !optionIDs[q.CorrectOptionID] {
				return Curriculum{}, fmt.Errorf("%s (id %q): correct_option_id %q does not match any option", where, q.QuestionID, q.CorrectOptionID)
			}
		}
		questionsByID[q.QuestionID] = q
	}

	// --- Chapters ------------------------------------------------------
	chapterIDs := map[string]bool{}
	chapterOrders := map[int]bool{}
	sortedChapters := append([]Chapter(nil), chapters...)
	sort.Slice(sortedChapters, func(i, j int) bool { return sortedChapters[i].Order < sortedChapters[j].Order })
	chaptersByID := map[string]Chapter{}
	chapterPosition := map[string]int{}
	for i, ch := range sortedChapters {
		where := fmt.Sprintf("chapter[%d]", i)
		if blank(ch.ChapterID) || chapterIDs[ch.ChapterID] {
			return Curriculum{}, fmt.Errorf("%s: empty or duplicate chapter_id %q", where, ch.ChapterID)
		}
		chapterIDs[ch.ChapterID] = true
		if ch.Order != i+1 {
			return Curriculum{}, fmt.Errorf("%s (id %q): order must be contiguous starting at 1, got %d at position %d", where, ch.ChapterID, ch.Order, i+1)
		}
		if chapterOrders[ch.Order] {
			return Curriculum{}, fmt.Errorf("%s (id %q): duplicate order %d", where, ch.ChapterID, ch.Order)
		}
		chapterOrders[ch.Order] = true
		if blank(ch.Title) {
			return Curriculum{}, fmt.Errorf("%s (id %q): title is required", where, ch.ChapterID)
		}
		if !oneOf(ch.Scope, "common", "class") {
			return Curriculum{}, fmt.Errorf("%s (id %q): invalid scope %q", where, ch.ChapterID, ch.Scope)
		}
		if ch.Scope == "class" {
			cfg, ok := classes[ch.ClassID]
			if !ok || cfg.Status == "coming_soon" {
				return Curriculum{}, fmt.Errorf("%s (id %q): class_id %q is not a playable wagon class", where, ch.ChapterID, ch.ClassID)
			}
		}
		chaptersByID[ch.ChapterID] = ch
		chapterPosition[ch.ChapterID] = i
	}

	// --- Lessons ---------------------------------------------------------
	lessonIDs := map[string]bool{}
	lessonsByID := map[string]Lesson{}
	for i, l := range lessons {
		where := fmt.Sprintf("lesson[%d]", i)
		if blank(l.LessonID) || lessonIDs[l.LessonID] {
			return Curriculum{}, fmt.Errorf("%s: empty or duplicate lesson_id %q", where, l.LessonID)
		}
		lessonIDs[l.LessonID] = true
		ownerChapter, ok := chaptersByID[l.ChapterID]
		if !ok {
			return Curriculum{}, fmt.Errorf("%s (id %q): chapter_id %q does not reference a known chapter", where, l.LessonID, l.ChapterID)
		}
		if blank(l.Title) || blank(l.BadgeID) {
			return Curriculum{}, fmt.Errorf("%s (id %q): title and badge_id are required", where, l.LessonID)
		}
		if len(l.TheoryQuestionIDs) == 0 {
			return Curriculum{}, fmt.Errorf("%s (id %q): theory_question_ids must be non-empty", where, l.LessonID)
		}
		if len(l.PracticeQuestionIDs) == 0 {
			return Curriculum{}, fmt.Errorf("%s (id %q): practice_question_ids must be non-empty", where, l.LessonID)
		}
		if !oneOf(l.CompletionRule, "visit_inspect", "scenario_result") {
			return Curriculum{}, fmt.Errorf("%s (id %q): invalid completion_rule %q", where, l.LessonID, l.CompletionRule)
		}
		switch l.CompletionRule {
		case "scenario_result":
			if blank(l.ScenarioID) {
				return Curriculum{}, fmt.Errorf("%s (id %q): scenario_result completion requires scenario_id", where, l.LessonID)
			}
			found := false
			for _, s := range catalog.Scenarios {
				if s.ID == l.ScenarioID {
					found = true
					break
				}
			}
			if !found {
				return Curriculum{}, fmt.Errorf("%s (id %q): scenario_id %q does not reference a known scenario", where, l.LessonID, l.ScenarioID)
			}
			if len(l.MandatoryEventIDs) == 0 {
				return Curriculum{}, fmt.Errorf("%s (id %q): scenario_result completion requires mandatory_event_ids", where, l.LessonID)
			}
		case "visit_inspect":
			if !blank(l.ScenarioID) {
				return Curriculum{}, fmt.Errorf("%s (id %q): visit_inspect completion must not set scenario_id", where, l.LessonID)
			}
			if len(l.RequiredAnchorIDs) == 0 {
				return Curriculum{}, fmt.Errorf("%s (id %q): visit_inspect completion requires required_anchor_ids", where, l.LessonID)
			}
			if len(l.RequiredObjectIDs) == 0 {
				return Curriculum{}, fmt.Errorf("%s (id %q): visit_inspect completion requires required_object_ids", where, l.LessonID)
			}
			cfg, ok := classes[ownerChapter.ClassID]
			if !ok {
				return Curriculum{}, fmt.Errorf("%s (id %q): visit_inspect completion requires owning chapter %q to have a valid class_id", where, l.LessonID, ownerChapter.ChapterID)
			}
			anchorSet := map[string]bool{}
			for _, a := range cfg.Anchors {
				anchorSet[a] = true
			}
			for _, a := range l.RequiredAnchorIDs {
				if !anchorSet[a] {
					return Curriculum{}, fmt.Errorf("%s (id %q): required_anchor_ids entry %q is not an anchor of wagon class %q", where, l.LessonID, a, ownerChapter.ClassID)
				}
			}
		}

		for _, qid := range l.TheoryQuestionIDs {
			q, ok := questionsByID[qid]
			if !ok {
				return Curriculum{}, fmt.Errorf("%s (id %q): theory_question_ids references unknown question %q", where, l.LessonID, qid)
			}
			if q.Phase != "theory" {
				return Curriculum{}, fmt.Errorf("%s (id %q): theory_question_ids references question %q with phase %q, want %q", where, l.LessonID, qid, q.Phase, "theory")
			}
		}
		for _, qid := range l.PracticeQuestionIDs {
			q, ok := questionsByID[qid]
			if !ok {
				return Curriculum{}, fmt.Errorf("%s (id %q): practice_question_ids references unknown question %q", where, l.LessonID, qid)
			}
			if q.Phase != "practice" {
				return Curriculum{}, fmt.Errorf("%s (id %q): practice_question_ids references question %q with phase %q, want %q", where, l.LessonID, qid, q.Phase, "practice")
			}
		}

		lessonsByID[l.LessonID] = l
	}

	// --- Chapter <-> lesson bidirectional link check ---------------------
	for _, ch := range sortedChapters {
		for _, lid := range ch.LessonIDs {
			l, ok := lessonsByID[lid]
			if !ok {
				return Curriculum{}, fmt.Errorf("chapter %q: lesson_ids references unknown lesson %q", ch.ChapterID, lid)
			}
			if l.ChapterID != ch.ChapterID {
				return Curriculum{}, fmt.Errorf("chapter %q: lesson_ids lists lesson %q, but that lesson's chapter_id is %q", ch.ChapterID, lid, l.ChapterID)
			}
		}
	}
	for _, l := range lessons {
		ch, ok := chaptersByID[l.ChapterID]
		if !ok {
			// Already reported above as an unknown chapter_id.
			continue
		}
		found := false
		for _, lid := range ch.LessonIDs {
			if lid == l.LessonID {
				found = true
				break
			}
		}
		if !found {
			return Curriculum{}, fmt.Errorf("lesson %q: chapter_id %q does not list this lesson in its lesson_ids", l.LessonID, l.ChapterID)
		}
	}

	// --- Sort before returning --------------------------------------------
	sortedLessons := append([]Lesson(nil), lessons...)
	sort.Slice(sortedLessons, func(i, j int) bool {
		pi, pj := chapterPosition[sortedLessons[i].ChapterID], chapterPosition[sortedLessons[j].ChapterID]
		if pi != pj {
			return pi < pj
		}
		return sortedLessons[i].Order < sortedLessons[j].Order
	})

	return Curriculum{
		Chapters:  sortedChapters,
		Lessons:   sortedLessons,
		Questions: questions,
	}, nil
}
