// Package session runs a learning session: grade one step, record what it proved,
// schedule what comes next (R1, R5, R6).
package session

import (
	"context"
	"fmt"
	"time"

	"jplings/internal/content"
	"jplings/internal/eval"
	"jplings/internal/grade"
	"jplings/internal/profile"
	"jplings/internal/schedule"
	"jplings/internal/speech"
)

// Runner grades steps and persists their consequences.
type Runner struct {
	Store     *profile.Store
	Client    speech.Client
	Evaluator eval.Evaluator
	// Now is injectable so retention and review scheduling are testable.
	Now   func() time.Time
	Voice string
	// Language defaults to ja.
	Language string
}

func (r *Runner) clock() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Clock exposes the runner's current time so callers persist cursors consistently.
func (r *Runner) Clock() time.Time { return r.clock() }

func (r *Runner) language() string {
	if r.Language == "" {
		return "ja"
	}
	return r.Language
}

// Result reports what one submission did.
type Result struct {
	Outcome grade.Outcome
	// Ungraded means no grader could score it and it was saved for later.
	Ungraded bool
	// MintedEvidence is true only when real credit was recorded.
	MintedEvidence bool
	// AIGraded marks feedback that came from the evaluator.
	AIGraded bool
}

// Submit grades one answer and records its consequences. It never auto-passes: a
// grading failure saves the response for the pending queue instead.
func (r *Runner) Submit(ctx context.Context, step *content.Step, answer string) (Result, error) {
	at := r.clock()

	if step.Kind == content.Speaking && step.Rubric != "" {
		return r.submitFreeForm(ctx, step, answer, at)
	}

	outcome := r.deterministic(step, answer)
	if err := r.record(step, outcome, at, evidenceSource(step)); err != nil {
		return Result{Outcome: outcome}, err
	}

	return Result{
		Outcome:        outcome,
		MintedEvidence: outcome.Correct,
	}, nil
}

func (r *Runner) deterministic(step *content.Step, answer string) grade.Outcome {
	if step.Kind == content.Interpretation {
		return grade.Meaning(step, answer)
	}
	if step.Kind == content.Speaking {
		// A confirmed transcript: word identity is graded, pronunciation is not.
		return grade.Spoken(step, answer)
	}
	return grade.Written(step, answer)
}

func evidenceSource(step *content.Step) string {
	if step.Kind == content.Speaking || step.Kind == content.Conversation {
		return "spoken"
	}
	return "typed"
}

// submitFreeForm grades open-ended production through the evaluator, and saves the
// response ungraded if that is unavailable for any reason.
func (r *Runner) submitFreeForm(ctx context.Context, step *content.Step, answer string, at time.Time) (Result, error) {
	if r.Evaluator == nil {
		return r.saveUngraded(step, answer, at, "no evaluator configured")
	}

	rubric, err := r.Evaluator.Evaluate(ctx, step.Prompt, answer)
	if err != nil {
		return r.saveUngraded(step, answer, at, err.Error())
	}

	outcome := grade.Outcome{
		// Understandable meaning plus usable grammar is the bar for credit; the two
		// dimensions stay independent so a right idea with broken grammar is not a pass.
		Correct:     rubric.Meaning >= 1 && rubric.Grammar >= 1,
		Explanation: fmt.Sprintf("%s (AI-graded: meaning %d, grammar %d, naturalness %d, register %d)", rubric.Feedback, rubric.Meaning, rubric.Grammar, rubric.Naturalness, rubric.Register),
	}

	if err := r.record(step, outcome, at, "llm"); err != nil {
		return Result{Outcome: outcome, AIGraded: true}, err
	}
	return Result{Outcome: outcome, AIGraded: true, MintedEvidence: outcome.Correct}, nil
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

// record writes evidence or an error for every concept the step teaches.
func (r *Runner) record(step *content.Step, outcome grade.Outcome, at time.Time, source string) error {
	dimension := grade.Dimension(step.Kind)
	if dimension == "" {
		return nil
	}

	for _, conceptID := range step.Concepts {
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

		if dimension == profile.Production {
			if err := r.mintGeneralization(conceptID, step.ID, at, source); err != nil {
				return err
			}
		}
		if err := r.scheduleRetentionIfReady(conceptID, at); err != nil {
			return err
		}
		if err := r.Store.ScheduleReview(schedule.Review(
			schedule.Initial(conceptID, at), schedule.Good, at,
		)); err != nil {
			return err
		}
	}
	return nil
}

// mintGeneralization treats production on a *different* stimulus as evidence of
// transfer. Repeating one sentence teaches nothing new, so it never counts.
func (r *Runner) mintGeneralization(conceptID, stimulusID string, at time.Time, source string) error {
	evidence, err := r.Store.Evidence(conceptID)
	if err != nil {
		return err
	}

	for _, existing := range evidence {
		if existing.Dimension != profile.Production || !existing.Correct {
			continue
		}
		if existing.StimulusID == "" || existing.StimulusID == stimulusID {
			continue
		}
		return r.Store.Record(profile.Evidence{
			ConceptID: conceptID, Dimension: profile.Generalization, Correct: true,
			StimulusID: stimulusID, Source: source, At: at,
		})
	}
	return nil
}

// scheduleRetentionIfReady queues the delayed, different-context check once both
// production and generalization are demonstrated (R6).
func (r *Runner) scheduleRetentionIfReady(conceptID string, at time.Time) error {
	mastered, missing, err := r.Store.Mastered(conceptID)
	if err != nil {
		return err
	}
	if mastered {
		return nil
	}
	// Only production and generalization matter for scheduling; retention is what
	// this call is about to add.
	for _, dimension := range missing {
		if dimension == profile.Retention {
			continue
		}
		return nil
	}

	exists, err := r.Store.HasRetentionCheck(conceptID)
	if err != nil || exists {
		return err
	}
	return r.Store.ScheduleReview(schedule.RetentionCard(conceptID, at))
}

// Speak delivers a prompt line, degrading to the written prompt when audio fails.
func (r *Runner) Speak(ctx context.Context, text string, play func([]byte) error) (speech.SpeakResult, error) {
	if r.Client == nil {
		return speech.SpeakResult{Degraded: true}, nil
	}
	return speech.Speak(ctx, r.Client, text, r.language(), r.Voice, play)
}
