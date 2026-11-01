package session

import (
	"context"
	"fmt"
	"time"

	"jplings/internal/content"
	"jplings/internal/convo"
	"jplings/internal/eval"
	"jplings/internal/grade"
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
	if r.Evaluator == nil {
		return r.saveUngraded(step, answer, at, "no evaluator configured")
	}

	rubric, err := r.Evaluator.Evaluate(ctx, step.Prompt, answer)
	if err != nil {
		return r.saveUngraded(step, answer, at, err.Error())
	}

	outcome := grade.Outcome{
		Correct:     rubric.Meaning >= 1 && rubric.Grammar >= 1,
		Explanation: fmt.Sprintf("%s (AI-graded: meaning %d, grammar %d, naturalness %d, register %d)", rubric.Feedback, rubric.Meaning, rubric.Grammar, rubric.Naturalness, rubric.Register),
	}

	if err := r.record(step, outcome, at, "llm", answer); err != nil {
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

		if source == "guided" {
			continue
		}
		if dimension == profile.Production {
			if err := r.mintGeneralization(conceptID, step.ID, at, source); err != nil {
				return err
			}
		}
		if err := r.scheduleRetentionIfReady(conceptID, at); err != nil {
			return err
		}
	}
	return nil
}

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

func (r *Runner) scheduleRetentionIfReady(conceptID string, at time.Time) error {
	mastered, missing, err := r.Store.Mastered(conceptID)
	if err != nil {
		return err
	}
	if mastered {
		return nil
	}
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
func (r *Runner) DueRetention() ([]schedule.Card, error) {
	return r.Store.DueRetention(r.clock())
}
func (r *Runner) GradeRetention(step *content.Step, answer string) (Result, error) {
	at := r.clock()
	outcome := r.deterministic(step, answer)

	for _, conceptID := range step.Concepts {
		if !outcome.Correct {
			if err := r.Store.AddError(conceptID, outcome.Explanation, at); err != nil {
				return Result{Outcome: outcome}, err
			}
			continue
		}
		err := r.Store.Record(profile.Evidence{
			ConceptID: conceptID, Dimension: profile.Retention, Correct: true,
			StimulusID: step.ID, Source: evidenceSource(step), At: at,
		})
		if err != nil {
			return Result{Outcome: outcome}, err
		}
		if err := r.Store.CompleteRetention(conceptID); err != nil {
			return Result{Outcome: outcome}, err
		}
	}
	return Result{Outcome: outcome, MintedEvidence: outcome.Correct}, nil
}
