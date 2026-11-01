package session

import (
	"context"
	"fmt"
	"time"

	"jplings/internal/content"
	"jplings/internal/convo"
	"jplings/internal/eval"
	"jplings/internal/grade"
	"jplings/internal/normalize"
	"jplings/internal/profile"
	"jplings/internal/schedule"
	"jplings/internal/speech"
)

type Runner struct {
	Store     *profile.Store
	Client    speech.Client
	Evaluator eval.Evaluator
	Now       func() time.Time
	Voice     string
	Language  string
}

func (r *Runner) clock() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Runner) Clock() time.Time { return r.clock() }

func (r *Runner) language() string {
	if r.Language == "" {
		return "ja"
	}
	return r.Language
}

type Result struct {
	Outcome        grade.Outcome
	Ungraded       bool
	MintedEvidence bool
	AIGraded       bool
}

func (r *Runner) Submit(ctx context.Context, step *content.Step, answer string) (Result, error) {
	at := r.clock()

	if step.Kind == content.Speaking && step.Rubric != "" {
		return r.submitFreeForm(ctx, step, answer, at)
	}

	outcome := r.deterministic(step, answer)
	if err := r.record(step, outcome, at, evidenceSource(step), answer); err != nil {
		return Result{Outcome: outcome}, err
	}

	return Result{
		Outcome:        outcome,
		MintedEvidence: outcome.Correct,
	}, nil
}

func (r *Runner) SubmitGuided(ctx context.Context, step *content.Step, answer string) (Result, error) {
	at := r.clock()
	outcome := r.deterministic(step, answer)
	if err := r.record(step, outcome, at, "guided", answer); err != nil {
		return Result{Outcome: outcome}, err
	}
	return Result{Outcome: outcome, MintedEvidence: outcome.Correct}, nil
}

func (r *Runner) Grade(step *content.Step, answer string) grade.Outcome {
	return r.deterministic(step, answer)
}

func (r *Runner) deterministic(step *content.Step, answer string) grade.Outcome {
	if step.Kind == content.Speaking {
		return grade.Spoken(step, answer)
	}
	if len(step.AcceptMeanings) > 0 {
		return grade.Meaning(step, answer)
	}
	return grade.Written(step, answer)
}

func evidenceSource(step *content.Step) string {
	if step.Kind == content.Speaking || step.Kind == content.Conversation {
		return "spoken"
	}
	return "typed"
}

func (r *Runner) submitFreeForm(ctx context.Context, step *content.Step, answer string, at time.Time) (Result, error) {
	outcome, reason, graded := r.freeFormOutcome(ctx, step, answer)
	if !graded {
		return r.saveUngraded(step, answer, at, reason)
	}

	if reproducesTarget(step, answer) {
		return Result{Outcome: outcome, AIGraded: true}, nil
	}
	if err := r.record(step, outcome, at, "llm", answer); err != nil {
		return Result{Outcome: outcome, AIGraded: true}, err
	}
	return Result{Outcome: outcome, AIGraded: true, MintedEvidence: outcome.Correct}, nil
}

func reproducesTarget(step *content.Step, answer string) bool {
	if step.Target == nil {
		return false
	}
	got := normalize.Surface(answer)
	for _, form := range []string{step.Target.Kanji, step.Target.Kana, step.Target.Romaji} {
		if form != "" && normalize.Surface(form) == got {
			return true
		}
	}
	return false
}

func (r *Runner) freeFormOutcome(ctx context.Context, step *content.Step, answer string) (grade.Outcome, string, bool) {
	if step.Rubric == "" {
		return grade.Outcome{}, "step has no rubric", false
	}
	if r.Evaluator == nil {
		return grade.Outcome{}, "no evaluator configured", false
	}

	rubric, err := r.Evaluator.Evaluate(ctx, step.Prompt, step.Rubric, answer)
	if err != nil {
		return grade.Outcome{}, err.Error(), false
	}

	return grade.Outcome{
		Correct:     rubric.Meaning >= 1 && rubric.Grammar >= 1,
		Explanation: fmt.Sprintf("%s (AI-graded: meaning %d, grammar %d, naturalness %d, register %d)", rubric.Feedback, rubric.Meaning, rubric.Grammar, rubric.Naturalness, rubric.Register),
	}, "", true
}

func (r *Runner) saveUngraded(step *content.Step, answer string, at time.Time, reason string) (Result, error) {
	if _, err := r.Store.EnqueuePending(step.ID, step.Prompt, answer, at); err != nil {
		return Result{}, err
	}
	return Result{
		Ungraded: true,
		Outcome: grade.Outcome{
			Explanation: fmt.Sprintf(
				"Saved for grading — %s. It will be graded next session and never counted until then.", reason,
			),
		},
	}, nil
}

func (r *Runner) record(step *content.Step, outcome grade.Outcome, at time.Time, source, answer string) error {
	dimension := grade.Dimension(step.Kind)
	if dimension == "" {
		return nil
	}

	concepts := step.Concepts
	if outcome.Correct {
		if matched := grade.MatchedConcepts(step, answer); len(matched) > 0 {
			concepts = matched
		}
	}
	for _, conceptID := range concepts {
		if !outcome.Correct {
			if err := r.Store.AddError(conceptID, outcome.Explanation, at); err != nil {
				return err
			}
			continue
		}

		err := r.Store.Record(profile.Evidence{
			ConceptID: conceptID, Dimension: dimension, Correct: true,
			StimulusID: step.ID, Source: source, At: at,
		})
		if err != nil {
			return err
		}
	}

	if outcome.Correct && source != "guided" && reviewable(step) && len(concepts) > 0 {
		if err := r.Store.EnsureCard(schedule.Initial(step.ID, concepts[0], at)); err != nil {
			return err
		}
	}
	return nil
}

// A card must be answerable by the deterministic graders. A scenario or a rubric step
// would come back every session as a prompt nothing can pass.
func reviewable(step *content.Step) bool {
	return step.Scenario == nil && step.Rubric == ""
}

func (r *Runner) Speak(ctx context.Context, text string, play func([]byte) error) (speech.SpeakResult, error) {
	if r.Client == nil {
		return speech.SpeakResult{Degraded: true}, nil
	}
	return speech.Speak(ctx, r.Client, text, r.language(), r.Voice, play)
}

func (r *Runner) GradeConversationTurn(step *content.Step, response convo.Response) (Result, error) {
	outcome := grade.Outcome{Explanation: step.Explanation}
	switch response.Verdict {
	case convo.Matched:
		outcome.Correct = true
	case convo.Tolerated:
		outcome.Explanation = response.Hint + " " + step.Explanation
	default:
		outcome.Explanation = "That does not answer this turn. " + step.Explanation
	}

	if err := r.record(step, outcome, r.clock(), "spoken", ""); err != nil {
		return Result{Outcome: outcome}, err
	}
	return Result{Outcome: outcome, MintedEvidence: outcome.Correct}, nil
}
func (r *Runner) MaySkip(conceptID string) (bool, error) { return r.Store.CanSkip(conceptID) }

const dueBlockLimit = 12

func (r *Runner) DueCards() ([]schedule.Card, error) {
	return r.Store.DueCards(r.clock(), dueBlockLimit)
}

// Review advances the card only when the learner actually answers it. Retention
// credit requires a due card whose stimulus was not already answered this session.
func (r *Runner) Review(card schedule.Card, step *content.Step, answer string, answeredThisSession bool) (Result, error) {
	at := r.clock()
	outcome := r.deterministic(step, answer)
	quality := schedule.Again
	if outcome.Correct {
		quality = schedule.Good
	}

	for _, conceptID := range step.Concepts {
		if !outcome.Correct {
			if err := r.Store.AddError(conceptID, outcome.Explanation, at); err != nil {
				return Result{Outcome: outcome}, err
			}
			continue
		}
		if answeredThisSession {
			continue
		}
		err := r.Store.Record(profile.Evidence{
			ConceptID: conceptID, Dimension: profile.Retention, Correct: true,
			StimulusID: step.ID, Source: evidenceSource(step), At: at,
		})
		if err != nil {
			return Result{Outcome: outcome}, err
		}
	}

	if err := r.Store.UpdateCard(schedule.Update(card, quality, at)); err != nil {
		return Result{Outcome: outcome}, err
	}
	return Result{Outcome: outcome, MintedEvidence: outcome.Correct && !answeredThisSession}, nil
}

func (r *Runner) RetryPending(ctx context.Context, lookup func(string) *content.Step) error {
	pending, err := r.Store.PendingToRetry()
	if err != nil {
		return err
	}
	for _, p := range pending {
		step := lookup(p.StepID)
		if step == nil {
			if err := r.Store.RetryPending(p.ID); err != nil {
				return err
			}
			continue
		}
		outcome, _, graded := r.freeFormOutcome(ctx, step, p.Response)
		if !graded {
			if r.Evaluator != nil {
				if err := r.Store.RetryPending(p.ID); err != nil {
					return err
				}
			}
			continue
		}
		if reproducesTarget(step, p.Response) {
			if err := r.Store.ResolvePending(p.ID); err != nil {
				return err
			}
			continue
		}
		if err := r.record(step, outcome, r.clock(), "llm", p.Response); err != nil {
			return err
		}
		if err := r.Store.ResolvePending(p.ID); err != nil {
			return err
		}
	}
	return nil
}
