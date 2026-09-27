package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var (
	ErrLessonNotFound      = errors.New("lesson not found")
	ErrLessonLocked        = errors.New("lesson is locked")
	ErrQuestionNotInLesson = errors.New("question is not part of this lesson")
	ErrTheoryNotPassed     = errors.New("theory has not been passed yet")
	ErrPracticeNotStarted  = errors.New("practice has not been started yet")
)

// lessonCompletionXP is the flat XP award for finishing a lesson (theory +
// practice + practice check all passed), per the design doc's "+20 lesson
// XP" rule.
const lessonCompletionXP = 20

// prizeCreditPerLesson is the flat prize-credit award for a lesson's first
// successful completion, per the separate, minimal prize-credit ledger's
// "+10 per B01/B02 first completion" rule. This is entirely independent of
// lessonCompletionXP -- it never touches players.total_xp.
const prizeCreditPerLesson = 10

// LearningService drives the player-facing curriculum: quiz questions,
// starting/finalizing lesson practice (wired to the wagon engine), and
// idempotent XP/badge awarding. It depends on the full repo.Store, not the
// narrow wagonStore interface -- same pattern as AdminService/ProfileService,
// which also talk to repo.Store directly rather than through WagonManager's
// internal interface.
type LearningService struct {
	store      repo.Store
	catalog    content.Catalog
	classes    content.WagonClasses
	curriculum content.Curriculum
	wagon      *WagonService
	manager    *WagonManager
	prize      *PrizeService
	push       *PushService
}

func NewLearningService(store repo.Store, catalog content.Catalog, classes content.WagonClasses, curriculum content.Curriculum, wagon *WagonService, manager *WagonManager, prize *PrizeService, push *PushService) *LearningService {
	return &LearningService{store: store, catalog: catalog, classes: classes, curriculum: curriculum, wagon: wagon, manager: manager, prize: prize, push: push}
}

func lessonByID(c content.Curriculum, id string) (content.Lesson, bool) {
	for _, l := range c.Lessons {
		if l.LessonID == id {
			return l, true
		}
	}
	return content.Lesson{}, false
}

func chapterByID(c content.Curriculum, id string) (content.Chapter, bool) {
	for _, ch := range c.Chapters {
		if ch.ChapterID == id {
			return ch, true
		}
	}
	return content.Chapter{}, false
}

func questionByID(c content.Curriculum, id string) (content.Question, bool) {
	for _, q := range c.Questions {
		if q.QuestionID == id {
			return q, true
		}
	}
	return content.Question{}, false
}

// nextLessonAfter returns the lesson immediately following lessonID in the
// curriculum's flattened chapter-then-lesson Order sequence (the same
// sequence lessonStatuses walks) -- i.e. the lesson that unlocks once
// lessonID is completed. ok is false if lessonID is not found or is already
// the last lesson in the sequence (nothing left to unlock).
func nextLessonAfter(curriculum content.Curriculum, lessonID string) (string, bool) {
	for i, l := range curriculum.Lessons {
		if l.LessonID == lessonID {
			if i+1 < len(curriculum.Lessons) {
				return curriculum.Lessons[i+1].LessonID, true
			}
			return "", false
		}
	}
	return "", false
}

func containsString(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

// LearningMap is the whole player-facing progression map: every chapter,
// even ones with no authored lessons yet, in Order.
type LearningMap struct {
	Chapters []LearningChapter `json:"chapters"`
}

type LearningChapter struct {
	ChapterID string                  `json:"chapter_id"`
	Title     string                  `json:"title"`
	Order     int                     `json:"order"`
	Lessons   []LearningLessonSummary `json:"lessons"`
}

type LearningLessonSummary struct {
	LessonID string `json:"lesson_id"`
	Title    string `json:"title"`
	Order    int    `json:"order"`
	Status   string `json:"status"` // "locked" | "unlocked" | "completed"
	BadgeID  string `json:"badge_id"`
}

// lessonStatuses computes every lesson's unlock status for playerID, in the
// curriculum's flattened chapter-then-lesson Order sequence (curriculum.Lessons
// is already sorted this way by content.LoadCurriculum/ParseCurriculum). The
// first lesson overall is always unlocked; every subsequent lesson unlocks
// once the previous lesson in that flattened sequence has CompletedAt set --
// not merely the previous lesson within the same chapter. This is the single
// source of truth for the lock rule, shared by GetMap, GetLesson,
// StartPractice and GetMyLearning so it's never reimplemented differently in
// two places.
func (s *LearningService) lessonStatuses(ctx context.Context, playerID uuid.UUID) (map[string]string, error) {
	statuses := make(map[string]string, len(s.curriculum.Lessons))
	prevCompleted := true
	for _, l := range s.curriculum.Lessons {
		progress, err := s.store.GetLessonProgress(ctx, playerID, l.LessonID)
		completed := false
		switch {
		case err == nil:
			completed = progress.CompletedAt != nil
		case errors.Is(err, repo.ErrNotFound):
			// Not started yet: not completed, handled below.
		default:
			return nil, err
		}
		switch {
		case completed:
			statuses[l.LessonID] = "completed"
		case prevCompleted:
			statuses[l.LessonID] = "unlocked"
		default:
			statuses[l.LessonID] = "locked"
		}
		prevCompleted = completed
	}
	return statuses, nil
}

// GetMap returns every chapter with its lessons and their unlock status for
// playerID. A chapter with zero authored lessons (chapters 2-6 today) still
// appears, with an empty lessons list, so the client can show it as
// "coming soon" rather than hide it entirely.
func (s *LearningService) GetMap(ctx context.Context, playerID uuid.UUID) (LearningMap, error) {
	statuses, err := s.lessonStatuses(ctx, playerID)
	if err != nil {
		return LearningMap{}, err
	}
	lessonsByChapter := map[string][]LearningLessonSummary{}
	for _, l := range s.curriculum.Lessons {
		lessonsByChapter[l.ChapterID] = append(lessonsByChapter[l.ChapterID], LearningLessonSummary{
			LessonID: l.LessonID, Title: l.Title, Order: l.Order, Status: statuses[l.LessonID], BadgeID: l.BadgeID,
		})
	}
	chapters := make([]LearningChapter, 0, len(s.curriculum.Chapters))
	for _, ch := range s.curriculum.Chapters {
		lessons := lessonsByChapter[ch.ChapterID]
		if lessons == nil {
			lessons = []LearningLessonSummary{}
		}
		chapters = append(chapters, LearningChapter{ChapterID: ch.ChapterID, Title: ch.Title, Order: ch.Order, Lessons: lessons})
	}
	return LearningMap{Chapters: chapters}, nil
}

// LessonQuestionOption is the redacted, player-facing view of one answer
// choice -- just id and display text, never which one is correct.
type LessonQuestionOption struct {
	OptionID string `json:"option_id"`
	Text     string `json:"text"`
}

// LessonQuestionView is the redacted, player-facing view of a
// content.Question. It is built field-by-field from content.Question,
// deliberately never by marshaling that type directly: content.Question
// carries CorrectOptionID and FeedbackByAnswer, which are the answer key and
// must never reach a client response unredacted.
type LessonQuestionView struct {
	QuestionID string                 `json:"question_id"`
	Phase      string                 `json:"phase"`
	Type       string                 `json:"type"`
	Prompt     string                 `json:"prompt"`
	Options    []LessonQuestionOption `json:"options,omitempty"`
}

// LessonDetail is the player-facing view of one lesson: its theory cards,
// redacted questions, and the player's current progress through it.
type LessonDetail struct {
	LessonID    string                `json:"lesson_id"`
	Title       string                `json:"title"`
	TheoryCards []string              `json:"theory_cards"`
	Questions   []LessonQuestionView  `json:"questions"`
	Progress    domain.LessonProgress `json:"progress"`
}

// redactQuestion builds the player-facing view of a content.Question.
// content.Question must NEVER be JSON-serialized directly to an HTTP
// response -- see the doc comment on content.Question and on
// LessonQuestionView.
func redactQuestion(q content.Question) LessonQuestionView {
	view := LessonQuestionView{QuestionID: q.QuestionID, Phase: q.Phase, Type: q.Type, Prompt: q.Prompt}
	for _, opt := range q.Options {
		view.Options = append(view.Options, LessonQuestionOption{OptionID: opt.OptionID, Text: opt.Text})
	}
	return view
}

// GetLesson returns lessonID's theory cards and redacted questions, plus the
// player's current LessonProgress, so the client knows what phase it's in.
// It refuses to serve a lesson the player hasn't unlocked yet (per
// lessonStatuses' rule) so a player can't theory-request a locked lesson.
func (s *LearningService) GetLesson(ctx context.Context, playerID uuid.UUID, lessonID string) (LessonDetail, error) {
	lesson, ok := lessonByID(s.curriculum, lessonID)
	if !ok {
		return LessonDetail{}, ErrLessonNotFound
	}
	statuses, err := s.lessonStatuses(ctx, playerID)
	if err != nil {
		return LessonDetail{}, err
	}
	if statuses[lessonID] == "locked" {
		return LessonDetail{}, ErrLessonLocked
	}
	progress, err := s.zeroValueOrProgress(ctx, playerID, lessonID)
	if err != nil {
		return LessonDetail{}, err
	}

	questions := make([]LessonQuestionView, 0, len(lesson.TheoryQuestionIDs)+len(lesson.PracticeQuestionIDs))
	for _, id := range append(append([]string{}, lesson.TheoryQuestionIDs...), lesson.PracticeQuestionIDs...) {
		q, ok := questionByID(s.curriculum, id)
		if !ok {
			return LessonDetail{}, fmt.Errorf("lesson %q references unknown question %q", lessonID, id)
		}
		questions = append(questions, redactQuestion(q))
	}

	return LessonDetail{
		LessonID:    lesson.LessonID,
		Title:       lesson.Title,
		TheoryCards: lesson.TheoryCards,
		Questions:   questions,
		Progress:    progress,
	}, nil
}

// zeroValueOrProgress loads a player's progress for lessonID, treating
// repo.ErrNotFound (no row yet) as "start from a zero-value progress row",
// not an error to the caller.
func (s *LearningService) zeroValueOrProgress(ctx context.Context, playerID uuid.UUID, lessonID string) (domain.LessonProgress, error) {
	progress, err := s.store.GetLessonProgress(ctx, playerID, lessonID)
	if err == nil {
		return progress, nil
	}
	if errors.Is(err, repo.ErrNotFound) {
		return domain.LessonProgress{PlayerID: playerID, LessonID: lessonID, ContentVersion: "1.0.0"}, nil
	}
	return domain.LessonProgress{}, err
}

// AnswerResult is the outcome of submitting one answer.
type AnswerResult struct {
	Correct   bool   `json:"correct"`
	Feedback  string `json:"feedback"`
	PhasePass bool   `json:"phase_pass"`
}

// SubmitAnswer records one answer to one question of lessonID and recomputes
// the relevant phase flag (TheoryPass for a theory question, PracticeCheckPass
// for a practice question) once every question in that phase has at least
// one correct answer on record.
func (s *LearningService) SubmitAnswer(ctx context.Context, playerID uuid.UUID, lessonID, questionID, optionID string) (AnswerResult, error) {
	lesson, ok := lessonByID(s.curriculum, lessonID)
	if !ok {
		return AnswerResult{}, ErrLessonNotFound
	}
	q, ok := questionByID(s.curriculum, questionID)
	if !ok {
		return AnswerResult{}, ErrQuestionNotInLesson
	}
	inTheory := containsString(lesson.TheoryQuestionIDs, questionID)
	inPractice := containsString(lesson.PracticeQuestionIDs, questionID)
	if !inTheory && !inPractice {
		return AnswerResult{}, ErrQuestionNotInLesson
	}

	correct := optionID == q.CorrectOptionID
	if err := s.store.RecordLessonAnswer(ctx, playerID, lessonID, questionID, optionID, correct); err != nil {
		return AnswerResult{}, err
	}

	progress, err := s.zeroValueOrProgress(ctx, playerID, lessonID)
	if err != nil {
		return AnswerResult{}, err
	}

	phaseQuestionIDs := lesson.TheoryQuestionIDs
	if inPractice {
		phaseQuestionIDs = lesson.PracticeQuestionIDs
	}
	phasePass := true
	for _, qid := range phaseQuestionIDs {
		ok, err := s.store.HasCorrectLessonAnswer(ctx, playerID, lessonID, qid)
		if err != nil {
			return AnswerResult{}, err
		}
		if !ok {
			phasePass = false
			break
		}
	}
	if inTheory {
		progress.TheoryPass = phasePass
	} else {
		progress.PracticeCheckPass = phasePass
	}
	if err := s.store.UpsertLessonProgress(ctx, progress); err != nil {
		return AnswerResult{}, err
	}

	feedback := q.FeedbackByAnswer[optionID]
	if feedback == "" {
		if correct {
			feedback = "Верно."
		} else {
			feedback = "Неверно, попробуй ещё раз."
		}
	}
	return AnswerResult{Correct: correct, Feedback: feedback, PhasePass: phasePass}, nil
}

// StartPractice starts the wagon session backing lessonID's practice phase.
// It requires theory to already be passed. For a "visit_inspect" lesson
// (e.g. B01) lesson.MandatoryEventIDs is empty, so StartScripted seeds no
// scripted scenario -- it behaves like a quiet Start. For a
// "scenario_result" lesson (e.g. B02) it seeds the lesson's scenario(s)
// deterministically.
func (s *LearningService) StartPractice(ctx context.Context, playerID uuid.UUID, lessonID string) (domain.Session, error) {
	lesson, ok := lessonByID(s.curriculum, lessonID)
	if !ok {
		return domain.Session{}, ErrLessonNotFound
	}
	progress, err := s.zeroValueOrProgress(ctx, playerID, lessonID)
	if err != nil {
		return domain.Session{}, err
	}
	if !progress.TheoryPass {
		return domain.Session{}, ErrTheoryNotPassed
	}
	chapter, ok := chapterByID(s.curriculum, lesson.ChapterID)
	if !ok {
		return domain.Session{}, fmt.Errorf("lesson %q: owning chapter %q not found", lessonID, lesson.ChapterID)
	}
	cfg, ok := s.classes[chapter.ClassID]
	if !ok || cfg.Status == "coming_soon" {
		return domain.Session{}, ErrWagonClassNotPlayable
	}
	if len(s.catalog.Passengers) == 0 {
		return domain.Session{}, ErrNoEligibleScenarios
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	seats := buildWagonSeats(s.catalog, cfg, rng)
	state := domain.WagonState{
		ClassID:           chapter.ClassID,
		LevelID:           "",
		RestrictedAnchors: append([]string(nil), cfg.RestrictedAnchors...),
		Seats:             seats,
		Player:            domain.WagonActor{At: cfg.ServicePointAnchor},
		CarriedItems:      []string{},
		StartedAt:         time.Now(),
		DurationS:         cfg.SessionDurationS,
	}
	sess, err := s.store.CreateWagonSession(ctx, playerID, state)
	if err != nil {
		return domain.Session{}, fmt.Errorf("create wagon session: %w", err)
	}
	s.manager.StartScripted(sess.ID, cfg, content.Level{}, state, lesson.MandatoryEventIDs)

	sessID := sess.ID
	progress.PracticeSessionID = &sessID
	if err := s.store.UpsertLessonProgress(ctx, progress); err != nil {
		return domain.Session{}, err
	}
	return sess, nil
}

// FinalizeResult is the outcome of finalizing a lesson's practice attempt.
type FinalizeResult struct {
	Completed      bool     `json:"completed"`
	Missing        []string `json:"missing,omitempty"` // subset of "theory_pass"|"practice_pass"|"practice_check_pass" still false
	AwardGranted   bool     `json:"award_granted"`     // false when this lesson was already awarded on an earlier call (idempotent repeat)
	XPAwarded      int      `json:"xp_awarded,omitempty"`
	BadgeID        string   `json:"badge_id,omitempty"`
	Debrief        string   `json:"debrief"`
	FoundAnchors   []string `json:"found_anchors,omitempty"`
	MissingAnchors []string `json:"missing_anchors,omitempty"`
	FoundObjects   []string `json:"found_objects,omitempty"`
	MissingObjects []string `json:"missing_objects,omitempty"`
	ScenarioPass   *bool    `json:"scenario_pass,omitempty"`
}

// FinalizePractice checks lessonID's practice attempt against its
// CompletionRule, persists the result, and -- only once all three of
// TheoryPass/PracticePass/PracticeCheckPass are true -- awards lesson
// completion exactly once (idempotent: repeat calls after the first award
// return Completed:true with AwardGranted:false, never double-applying XP).
func (s *LearningService) FinalizePractice(ctx context.Context, playerID uuid.UUID, lessonID string) (FinalizeResult, error) {
	lesson, ok := lessonByID(s.curriculum, lessonID)
	if !ok {
		return FinalizeResult{}, ErrLessonNotFound
	}
	progress, err := s.store.GetLessonProgress(ctx, playerID, lessonID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return FinalizeResult{}, ErrPracticeNotStarted
		}
		return FinalizeResult{}, err
	}
	if progress.PracticeSessionID == nil {
		return FinalizeResult{}, ErrPracticeNotStarted
	}
	sess, err := s.store.GetSession(ctx, *progress.PracticeSessionID)
	if err != nil {
		return FinalizeResult{}, err
	}

	result := FinalizeResult{Debrief: lesson.DebriefIntro}
	var practicePass bool
	switch lesson.CompletionRule {
	case "visit_inspect":
		visited := map[string]bool{}
		inspected := map[string]bool{}
		if sess.WagonState != nil {
			for _, a := range sess.WagonState.VisitedAnchors {
				visited[a] = true
			}
			for _, o := range sess.WagonState.InspectedObjects {
				inspected[o] = true
			}
		}
		allFound := true
		for _, a := range lesson.RequiredAnchorIDs {
			if visited[a] {
				result.FoundAnchors = append(result.FoundAnchors, a)
			} else {
				result.MissingAnchors = append(result.MissingAnchors, a)
				allFound = false
			}
		}
		for _, o := range lesson.RequiredObjectIDs {
			if inspected[o] {
				result.FoundObjects = append(result.FoundObjects, o)
			} else {
				result.MissingObjects = append(result.MissingObjects, o)
				allFound = false
			}
		}
		practicePass = allFound
	case "scenario_result":
		// Pass iff no situation in the session resolved with outcome "fail"
		// -- the exact same rule as WagonService.AdvanceIfPassed and
		// AdminService.wagonProgression use for the standard wagon-level
		// gate, for consistency. A fuller causal debrief (what was an
		// observed signal vs. what the passenger actually said vs. what was
		// only assumed) could read this session's message/scoring log
		// later; that's out of scope for this task.
		situations, err := s.store.ListSituationsBySession(ctx, sess.ID)
		if err != nil {
			return FinalizeResult{}, err
		}
		pass := true
		for _, sit := range situations {
			if sit.Outcome != nil && *sit.Outcome == "fail" {
				pass = false
				break
			}
		}
		practicePass = pass
		result.ScenarioPass = &pass
	default:
		return FinalizeResult{}, fmt.Errorf("lesson %q: unknown completion_rule %q", lessonID, lesson.CompletionRule)
	}

	progress.PracticePass = practicePass
	if err := s.store.UpsertLessonProgress(ctx, progress); err != nil {
		return FinalizeResult{}, err
	}

	if !(progress.TheoryPass && progress.PracticePass && progress.PracticeCheckPass) {
		if !progress.TheoryPass {
			result.Missing = append(result.Missing, "theory_pass")
		}
		if !progress.PracticePass {
			result.Missing = append(result.Missing, "practice_pass")
		}
		if !progress.PracticeCheckPass {
			result.Missing = append(result.Missing, "practice_check_pass")
		}
		result.Completed = false
		return result, nil
	}

	granted, err := s.store.AwardLessonCompletion(ctx, playerID, lessonID, lessonCompletionXP, lesson.BadgeID)
	if err != nil {
		return FinalizeResult{}, err
	}
	result.Completed = true
	result.AwardGranted = granted
	result.BadgeID = lesson.BadgeID
	if granted {
		result.XPAwarded = lessonCompletionXP
		now := time.Now()
		progress.CompletedAt = &now
		if err := s.store.UpsertLessonProgress(ctx, progress); err != nil {
			return FinalizeResult{}, err
		}
		// Prize-credit ledger is a secondary bookkeeping concern, entirely
		// independent of XP/badges/the leaderboard: a failure here must not
		// turn this otherwise-successful FinalizePractice call into an
		// error, same precedent as FinishSession's handling of
		// AdvanceIfPassed's error in handler/session.go.
		if _, err := s.prize.AwardLessonPrize(ctx, playerID, lessonID, prizeCreditPerLesson); err != nil {
			slog.Error("prize credit award failed", "player_id", playerID, "lesson_id", lessonID, "error", err)
		}
		// Same "secondary bookkeeping must not break the primary success
		// response" precedent as the prize-credit award above and
		// FinishSession's handling of AdvanceIfPassed's error in
		// handler/session.go: a lesson_unlocked push failure must never turn
		// this otherwise-successful FinalizePractice call into an error.
		if next, ok := nextLessonAfter(s.curriculum, lessonID); ok {
			if err := s.push.NotifyLessonUnlocked(ctx, playerID, next); err != nil {
				slog.Error("lesson unlocked push failed", "player_id", playerID, "unlocked_lesson_id", next, "error", err)
			}
		}
	}
	return result, nil
}

// MyLearningLesson is one lesson's entry in the player's own learning
// summary (GET /api/me/learning).
type MyLearningLesson struct {
	LessonID    string     `json:"lesson_id"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// MyLearning is the player's own learning summary. PrizeBalance and
// PrizeNextExpiry are the same wire shape task 3 stubbed at zero/null;
// GetMyLearning now fills them from the real prize-credit ledger (task 4).
type MyLearning struct {
	Lessons         []MyLearningLesson `json:"lessons"`
	PrizeBalance    int                `json:"prize_balance"`
	PrizeNextExpiry *time.Time         `json:"prize_next_expiry"`
	ShirtThreshold  int                `json:"prize_shirt_threshold"`
	ShirtProgress   float64            `json:"prize_shirt_progress"`
}

func (s *LearningService) GetMyLearning(ctx context.Context, playerID uuid.UUID) (MyLearning, error) {
	statuses, err := s.lessonStatuses(ctx, playerID)
	if err != nil {
		return MyLearning{}, err
	}
	lessons := make([]MyLearningLesson, 0, len(s.curriculum.Lessons))
	for _, l := range s.curriculum.Lessons {
		progress, err := s.store.GetLessonProgress(ctx, playerID, l.LessonID)
		var completedAt *time.Time
		switch {
		case err == nil:
			completedAt = progress.CompletedAt
		case errors.Is(err, repo.ErrNotFound):
		default:
			return MyLearning{}, err
		}
		lessons = append(lessons, MyLearningLesson{LessonID: l.LessonID, Status: statuses[l.LessonID], CompletedAt: completedAt})
	}
	prize, err := s.prize.Balance(ctx, playerID)
	if err != nil {
		return MyLearning{}, err
	}
	return MyLearning{
		Lessons:         lessons,
		PrizeBalance:    prize.Balance,
		PrizeNextExpiry: prize.NextExpiryAt,
		ShirtThreshold:  prize.ShirtThreshold,
		ShirtProgress:   prize.ShirtProgress,
	}, nil
}
