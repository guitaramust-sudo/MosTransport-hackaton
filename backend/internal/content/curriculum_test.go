package content

import "testing"

func TestLoadCurriculum(t *testing.T) {
	cur, err := LoadCurriculum()
	if err != nil {
		t.Fatal(err)
	}
	if len(cur.Chapters) != 6 {
		t.Fatalf("expected 6 chapters, got %d", len(cur.Chapters))
	}
	for i, ch := range cur.Chapters {
		if ch.Order != i+1 {
			t.Fatalf("chapters must be sorted by order, got %+v at position %d", ch, i)
		}
	}
	if got := cur.Chapters[0].LessonIDs; len(got) != 2 || got[0] != "B01" || got[1] != "B02" {
		t.Fatalf("chapter 1 lesson_ids = %v, want [B01 B02]", got)
	}
	for i := 1; i < 6; i++ {
		if len(cur.Chapters[i].LessonIDs) != 0 {
			t.Fatalf("chapter %q expected zero lessons, got %v", cur.Chapters[i].ChapterID, cur.Chapters[i].LessonIDs)
		}
	}
	if len(cur.Lessons) != 2 {
		t.Fatalf("expected 2 lessons, got %d", len(cur.Lessons))
	}
	if len(cur.Questions) != 8 {
		t.Fatalf("expected 8 questions, got %d", len(cur.Questions))
	}
}

func TestCurriculumLessonsReferenceRealQuestionsWithMatchingPhase(t *testing.T) {
	cur, err := LoadCurriculum()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Question{}
	for _, q := range cur.Questions {
		byID[q.QuestionID] = q
	}
	for _, l := range cur.Lessons {
		for _, qid := range l.TheoryQuestionIDs {
			q, ok := byID[qid]
			if !ok || q.Phase != "theory" {
				t.Fatalf("lesson %q theory question %q missing or wrong phase: %+v", l.LessonID, qid, q)
			}
		}
		for _, qid := range l.PracticeQuestionIDs {
			q, ok := byID[qid]
			if !ok || q.Phase != "practice" {
				t.Fatalf("lesson %q practice question %q missing or wrong phase: %+v", l.LessonID, qid, q)
			}
		}
	}
}

func TestCurriculumB01RequiredAnchorsExistOnFirstClass(t *testing.T) {
	cur, err := LoadCurriculum()
	if err != nil {
		t.Fatal(err)
	}
	classes, err := LoadWagonClasses()
	if err != nil {
		t.Fatal(err)
	}
	anchors := map[string]bool{}
	for _, a := range classes["first"].Anchors {
		anchors[a] = true
	}
	var b01 Lesson
	found := false
	for _, l := range cur.Lessons {
		if l.LessonID == "B01" {
			b01 = l
			found = true
		}
	}
	if !found {
		t.Fatal("lesson B01 not found")
	}
	if len(b01.RequiredAnchorIDs) == 0 {
		t.Fatal("B01 required_anchor_ids is empty")
	}
	for _, a := range b01.RequiredAnchorIDs {
		if !anchors[a] {
			t.Fatalf("B01 required anchor %q is not an anchor of the first wagon class %v", a, classes["first"].Anchors)
		}
	}
}

func TestCurriculumB02ReferencesRealScenario(t *testing.T) {
	cur, err := LoadCurriculum()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	var b02 Lesson
	found := false
	for _, l := range cur.Lessons {
		if l.LessonID == "B02" {
			b02 = l
			found = true
		}
	}
	if !found {
		t.Fatal("lesson B02 not found")
	}
	if b02.ScenarioID != "cold" {
		t.Fatalf("B02 scenario_id = %q, want %q", b02.ScenarioID, "cold")
	}
	scenarioFound := false
	for _, s := range catalog.Scenarios {
		if s.ID == b02.ScenarioID {
			scenarioFound = true
		}
	}
	if !scenarioFound {
		t.Fatalf("scenario %q referenced by B02 not present in catalog", b02.ScenarioID)
	}
}

// minimalFixture returns valid, minimal chapters/lessons/questions JSON that
// mutate() can tweak to construct an invalid fixture, plus the catalog and
// wagon classes needed to parse it.
func minimalFixture() (chaptersRaw, lessonsRaw, questionsRaw []byte, catalog Catalog, classes WagonClasses, err error) {
	catalog, err = Load()
	if err != nil {
		return
	}
	classes, err = LoadWagonClasses()
	if err != nil {
		return
	}
	chaptersRaw = []byte(`[{"chapter_id":"c1","order":1,"title":"C1","lesson_ids":["l1"],"scope":"class","class_id":"first","content_version":"1.0.0","validation_status":"draft"}]`)
	lessonsRaw = []byte(`[{"lesson_id":"l1","chapter_id":"c1","order":1,"title":"L1","badge_id":"b1","estimated_min":5,"theory_cards":["x"],"theory_question_ids":["q1"],"required_anchor_ids":["sanitary_zone"],"required_object_ids":["o1"],"practice_question_ids":["q2"],"completion_rule":"visit_inspect","debrief_intro":"d","validation_status":"draft"}]`)
	questionsRaw = []byte(`[{"question_id":"q1","phase":"theory","type":"single_choice","prompt":"p1","options":[{"option_id":"a","text":"A"},{"option_id":"b","text":"B"}],"correct_option_id":"a","validation_status":"draft"},{"question_id":"q2","phase":"practice","type":"single_choice","prompt":"p2","options":[{"option_id":"a","text":"A"},{"option_id":"b","text":"B"}],"correct_option_id":"a","validation_status":"draft"}]`)
	return
}

func TestParseCurriculumAcceptsMinimalFixture(t *testing.T) {
	chaptersRaw, lessonsRaw, questionsRaw, catalog, classes, err := minimalFixture()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseCurriculum(chaptersRaw, lessonsRaw, questionsRaw, catalog, classes); err != nil {
		t.Fatalf("minimal fixture should parse cleanly: %v", err)
	}
}

func TestParseCurriculumRejectsUnknownQuestionReference(t *testing.T) {
	chaptersRaw, lessonsRaw, _, catalog, classes, err := minimalFixture()
	if err != nil {
		t.Fatal(err)
	}
	lessonsRaw = []byte(`[{"lesson_id":"l1","chapter_id":"c1","order":1,"title":"L1","badge_id":"b1","estimated_min":5,"theory_cards":["x"],"theory_question_ids":["missing"],"required_anchor_ids":["sanitary_zone"],"required_object_ids":["o1"],"practice_question_ids":["q2"],"completion_rule":"visit_inspect","debrief_intro":"d","validation_status":"draft"}]`)
	questionsRaw := []byte(`[{"question_id":"q2","phase":"practice","type":"single_choice","prompt":"p2","options":[{"option_id":"a","text":"A"},{"option_id":"b","text":"B"}],"correct_option_id":"a","validation_status":"draft"}]`)
	if _, err := ParseCurriculum(chaptersRaw, lessonsRaw, questionsRaw, catalog, classes); err == nil {
		t.Fatal("expected an error for an unknown question reference")
	}
}

func TestParseCurriculumRejectsPhaseMismatch(t *testing.T) {
	chaptersRaw, lessonsRaw, questionsRaw, catalog, classes, err := minimalFixture()
	if err != nil {
		t.Fatal(err)
	}
	// q2 is phase practice but referenced under theory_question_ids.
	lessonsRaw = []byte(`[{"lesson_id":"l1","chapter_id":"c1","order":1,"title":"L1","badge_id":"b1","estimated_min":5,"theory_cards":["x"],"theory_question_ids":["q2"],"required_anchor_ids":["sanitary_zone"],"required_object_ids":["o1"],"practice_question_ids":["q1"],"completion_rule":"visit_inspect","debrief_intro":"d","validation_status":"draft"}]`)
	if _, err := ParseCurriculum(chaptersRaw, lessonsRaw, questionsRaw, catalog, classes); err == nil {
		t.Fatal("expected an error for a phase mismatch")
	}
}

func TestParseCurriculumRejectsBadCorrectOptionID(t *testing.T) {
	chaptersRaw, lessonsRaw, _, catalog, classes, err := minimalFixture()
	if err != nil {
		t.Fatal(err)
	}
	questionsRaw := []byte(`[{"question_id":"q1","phase":"theory","type":"single_choice","prompt":"p1","options":[{"option_id":"a","text":"A"},{"option_id":"b","text":"B"}],"correct_option_id":"zzz","validation_status":"draft"},{"question_id":"q2","phase":"practice","type":"single_choice","prompt":"p2","options":[{"option_id":"a","text":"A"},{"option_id":"b","text":"B"}],"correct_option_id":"a","validation_status":"draft"}]`)
	if _, err := ParseCurriculum(chaptersRaw, lessonsRaw, questionsRaw, catalog, classes); err == nil {
		t.Fatal("expected an error for a correct_option_id with no matching option")
	}
}

func TestParseCurriculumRejectsOrphanChapterLessonLink(t *testing.T) {
	_, lessonsRaw, questionsRaw, catalog, classes, err := minimalFixture()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("chapter lists unknown lesson", func(t *testing.T) {
		chaptersRaw := []byte(`[{"chapter_id":"c1","order":1,"title":"C1","lesson_ids":["l1","ghost"],"scope":"class","class_id":"first","content_version":"1.0.0","validation_status":"draft"}]`)
		if _, err := ParseCurriculum(chaptersRaw, lessonsRaw, questionsRaw, catalog, classes); err == nil {
			t.Fatal("expected an error for a chapter listing a nonexistent lesson_id")
		}
	})
	t.Run("lesson not wired into its chapter", func(t *testing.T) {
		chaptersRaw := []byte(`[{"chapter_id":"c1","order":1,"title":"C1","lesson_ids":[],"scope":"class","class_id":"first","content_version":"1.0.0","validation_status":"draft"}]`)
		if _, err := ParseCurriculum(chaptersRaw, lessonsRaw, questionsRaw, catalog, classes); err == nil {
			t.Fatal("expected an error for a lesson whose chapter doesn't list it")
		}
	})
}
