// Package content defines the declarative curriculum schema and its validation.
//
// Content files are read-only to the engine: they declare units, lessons and
// steps, and the engine owns no pedagogy (D7, R2).
package content

import (
	"fmt"
	"strings"
)

// Kind is an exercise kind (R3).
type Kind string

const (
	Construction   Kind = "construction"
	Transformation Kind = "transformation"
	Completion     Kind = "completion"
	Interpretation Kind = "interpretation"
	Listening      Kind = "listening"
	Speaking       Kind = "speaking"
	Conversation   Kind = "conversation"
)

var kinds = map[Kind]bool{
	Construction: true, Transformation: true, Completion: true,
	Interpretation: true, Listening: true, Speaking: true, Conversation: true,
}

// Text is an authored Japanese string. Kana and romaji are always authored, never
// generated: the romaji rung of the modality ladder is content, not romanization (D7).
type Text struct {
	Kanji  string `yaml:"kanji,omitempty"`
	Kana   string `yaml:"kana"`
	Romaji string `yaml:"romaji"`
}

// OffTarget is valid Japanese that answers a different question, such as 飲めない
// for "negative of 飲む". It earns no credit and is reported as a meaning error (D3).
type OffTarget struct {
	Text    Text   `yaml:"text"`
	Meaning string `yaml:"meaning"`
}

// Turn is one line of a scripted conversation (D4).
type Turn struct {
	Speaker string   `yaml:"speaker"` // "npc" or "learner"
	Line    *Text    `yaml:"line,omitempty"`
	Accept  []Text   `yaml:"accept,omitempty"`
	Hints   []string `yaml:"hints,omitempty"`
}

// Scenario is a scripted multi-turn conversation with bounded variation (D4).
type Scenario struct {
	Turns []Turn `yaml:"turns"`
}

// Step is one exercise. The fields are a union rather than one type per kind so
// that lint can report every authoring mistake at once; Validate enforces the
// fields each kind actually requires.
type Step struct {
	ID       string   `yaml:"id"`
	Kind     Kind     `yaml:"kind"`
	Concepts []string `yaml:"concepts,omitempty"`
	Prompt   string   `yaml:"prompt,omitempty"`
	Target   *Text    `yaml:"target,omitempty"`
	// Audio overrides the synthesized prompt. Listening exercises synthesize their
	// target at runtime unless a pre-recorded clip is supplied here.
	Audio            string      `yaml:"audio,omitempty"`
	AudioSource      string      `yaml:"audio_source,omitempty"`
	AudioLicense     string      `yaml:"audio_license,omitempty"`
	AudioAttribution string      `yaml:"audio_attribution,omitempty"`
	Accept           []Text      `yaml:"accept,omitempty"`
	AcceptMeanings   []string    `yaml:"accept_meanings,omitempty"`
	OffTarget        []OffTarget `yaml:"off_target,omitempty"`
	Rubric           string      `yaml:"rubric,omitempty"`
	Explanation      string      `yaml:"explanation,omitempty"`
	Scenario         *Scenario   `yaml:"scenario,omitempty"`
}

// Concept is a trackable idea: the unit of evidence and mastery (R5, R6).
type Concept struct {
	ID    string `yaml:"id"`
	Label string `yaml:"label,omitempty"`
}

// Lesson groups steps that teach one thread.
type Lesson struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
	Steps []Step `yaml:"steps"`
}

// Unit is one content file's top-level curriculum piece.
type Unit struct {
	ID       string    `yaml:"id"`
	Title    string    `yaml:"title"`
	Concepts []Concept `yaml:"concepts,omitempty"`
	Lessons  []Lesson  `yaml:"lessons"`
}

// Issue is one lint finding, anchored to the smallest path that names it.
type Issue struct {
	Path string
	Msg  string
}

func (i Issue) String() string { return i.Path + ": " + i.Msg }

// Validate reports every schema violation in the unit.
func (u *Unit) Validate() []Issue {
	var issues []Issue
	add := func(path, format string, args ...any) {
		issues = append(issues, Issue{Path: path, Msg: fmt.Sprintf(format, args...)})
	}

	if u.ID == "" {
		add("unit", "missing id")
	}
	if u.Title == "" {
		add(path("", "title"), "missing title")
	}

	concepts := map[string]bool{}
	for i, c := range u.Concepts {
		p := path("", fmt.Sprintf("concepts[%d]", i))
		if c.ID == "" {
			add(p, "missing id")
			continue
		}
		if concepts[c.ID] {
			add(p, "duplicate concept id %q", c.ID)
		}
		concepts[c.ID] = true
	}

	if len(u.Lessons) == 0 {
		add("unit", "has no lessons")
	}

	lessonIDs := map[string]bool{}
	stepIDs := map[string]bool{}
	for li, lesson := range u.Lessons {
		lp := path("", fmt.Sprintf("lessons[%d]", li))
		if lesson.ID == "" {
			add(lp, "missing id")
		} else {
			if lessonIDs[lesson.ID] {
				add(lp, "duplicate lesson id %q", lesson.ID)
			}
			lessonIDs[lesson.ID] = true
			lp = path(lesson.ID, "")
		}
		if lesson.Title == "" {
			add(path(lesson.ID, "title"), "missing title")
		}
		if len(lesson.Steps) == 0 {
			add(lp, "has no steps")
		}
		for si := range lesson.Steps {
			step := &lesson.Steps[si]
			sp := path(lesson.ID, step.ID)
			if step.ID == "" {
				sp = path(lesson.ID, fmt.Sprintf("steps[%d]", si))
				add(sp, "missing id")
			} else {
				if stepIDs[step.ID] {
					add(sp, "duplicate step id %q", step.ID)
				}
				stepIDs[step.ID] = true
			}
			step.validate(sp, concepts, add)
		}
	}

	return issues
}

func (s *Step) validate(where string, concepts map[string]bool, add func(path, format string, args ...any)) {
	if !kinds[s.Kind] {
		add(where, "unknown kind %q", s.Kind)
		return
	}
	for i, id := range s.Concepts {
		if !concepts[id] {
			add(path(where, fmt.Sprintf("concepts[%d]", i)), "undeclared concept %q", id)
		}
	}
	for i := range s.Accept {
		s.Accept[i].validate(path(where, fmt.Sprintf("accept[%d]", i)), add)
	}
	for i, ot := range s.OffTarget {
		otp := path(where, fmt.Sprintf("off_target[%d]", i))
		ot.Text.validate(otp, add)
		if ot.Meaning == "" {
			add(otp, "missing meaning")
		}
	}
	if s.Target != nil {
		s.Target.validate(path(where, "target"), add)
	}
	// Authentic clips must carry their provenance: an unlicensed clip may never be
	// reused outside its source (handoff §8A and the §9 Q9 disposition).
	if s.AudioSource != "" || s.AudioLicense != "" || s.AudioAttribution != "" {
		if s.Audio == "" {
			add(where, "audio provenance given without an audio file")
		}
		for name, value := range map[string]string{
			"audio_source":      s.AudioSource,
			"audio_license":     s.AudioLicense,
			"audio_attribution": s.AudioAttribution,
		} {
			if value == "" {
				add(where, "audio provenance must include %s", name)
			}
		}
	}
	if s.Scenario != nil {
		s.Scenario.validate(path(where, "scenario"), add)
	}

	switch s.Kind {
	case Construction:
		s.requirePrompt(where, add)
		s.requireAccept(where, add)
		s.requireExplanation(where, add)
	case Transformation, Completion:
		s.requirePrompt(where, add)
		s.requireTarget(where, add)
		s.requireAccept(where, add)
		s.requireExplanation(where, add)
	case Interpretation:
		s.requirePrompt(where, add)
		s.requireTarget(where, add)
		if len(s.AcceptMeanings) == 0 {
			add(where, "interpretation needs at least one accept_meaning")
		}
		s.requireExplanation(where, add)
	case Listening:
		s.requirePrompt(where, add)
		s.requireTarget(where, add)
		s.requireAccept(where, add)
		s.requireExplanation(where, add)
	case Speaking:
		s.requirePrompt(where, add)
		// A spoken turn is graded deterministically from accepted answers
		// (numbers, prices) or by the rubric evaluator (free form) — exactly one,
		// so grading responsibility is never ambiguous (D3, D7).
		switch {
		case len(s.Accept) > 0 && s.Rubric != "":
			add(where, "speaking has both accept and rubric; pick one grading path")
		case len(s.Accept) == 0 && s.Rubric == "":
			add(where, "speaking needs either accept answers or a rubric")
		}
	case Conversation:
		s.requirePrompt(where, add)
		if s.Scenario == nil {
			add(where, "conversation needs a scenario")
		}
	}
}

func (s *Scenario) validate(where string, add func(path, format string, args ...any)) {
	if s == nil {
		return
	}
	if len(s.Turns) == 0 {
		add(where, "has no turns")
	}
	for i := range s.Turns {
		t := &s.Turns[i]
		tp := path(where, fmt.Sprintf("turns[%d]", i))
		switch t.Speaker {
		case "npc":
			if t.Line == nil {
				add(tp, "npc turn needs a line")
			} else {
				t.Line.validate(tp, add)
			}
		case "learner":
			if len(t.Accept) == 0 {
				add(tp, "learner turn needs at least one accepted answer")
			}
			for j := range t.Accept {
				t.Accept[j].validate(path(tp, fmt.Sprintf("accept[%d]", j)), add)
			}
		default:
			add(tp, "speaker must be \"npc\" or \"learner\", got %q", t.Speaker)
		}
	}
}

func (t *Text) validate(where string, add func(path, format string, args ...any)) {
	if t == nil {
		return
	}
	if t.Kana == "" {
		add(where, "missing kana reading")
	}
	if t.Romaji == "" {
		add(where, "missing authored romaji")
	}
}

func (s *Step) requirePrompt(where string, add func(string, string, ...any)) {
	if s.Prompt == "" {
		add(where, "%s needs a prompt", s.Kind)
	}
}

func (s *Step) requireTarget(where string, add func(string, string, ...any)) {
	if s.Target == nil {
		add(where, "%s needs a target", s.Kind)
	}
}

func (s *Step) requireAccept(where string, add func(string, string, ...any)) {
	if len(s.Accept) == 0 {
		add(where, "%s needs at least one accepted answer", s.Kind)
	}
}

func (s *Step) requireExplanation(where string, add func(string, string, ...any)) {
	if s.Explanation == "" {
		add(where, "%s needs an explanation: feedback names the error, never bare correct/incorrect", s.Kind)
	}
}

// path joins a lesson/step locator with a field, skipping empties, so findings
// point at the smallest thing that names them.
func path(base, field string) string {
	switch {
	case base == "":
		return field
	case field == "":
		return base
	default:
		return strings.TrimSuffix(base, "/") + "/" + field
	}
}
