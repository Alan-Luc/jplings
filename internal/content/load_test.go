package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validUnit = `
id: u
title: Unit
concepts:
  - id: c1
lessons:
  - id: l1
    title: Lesson
    steps:
      - id: s1
        kind: construction
        concepts: [c1]
        prompt: Say it.
        accept:
          - kana: あ
            romaji: a
        explanation: because
`

func writeUnit(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValidUnit(t *testing.T) {
	units, issues, err := Load(writeUnit(t, "u.yaml", validUnit))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
	if len(units) != 1 || units[0].ID != "u" {
		t.Fatalf("expected one unit with id u, got %v", units)
	}
}

func TestLoadReportsSchemaViolations(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing kana reading",
			body: strings.Replace(validUnit, "          - kana: あ\n            romaji: a", "          - romaji: a", 1),
			want: "missing kana reading",
		},
		{
			name: "missing authored romaji",
			body: strings.Replace(validUnit, "          - kana: あ\n            romaji: a", "          - kana: あ", 1),
			want: "missing authored romaji",
		},
		{
			name: "unknown field",
			body: strings.Replace(validUnit, "        prompt: Say it.", "        promt: Say it.", 1),
			want: "promt",
		},
		{
			name: "duplicate step id",
			body: strings.Replace(validUnit, "      - id: s1\n        kind: construction",
				"      - id: s1\n        kind: interpretation\n        target:\n          kana: あ\n          romaji: a\n        accept_meanings: [x]\n      - id: s1\n        kind: construction", 1),
			want: `duplicate step id "s1"`,
		},
		{
			name: "undeclared concept",
			body: strings.Replace(validUnit, "concepts: [c1]", "concepts: [nope]", 1),
			want: `undeclared concept "nope"`,
		},
		{
			name: "construction without accepted answer",
			body: strings.Replace(validUnit, "        accept:\n          - kana: あ\n            romaji: a\n", "", 1),
			want: "construction needs at least one accepted answer",
		},
		{
			name: "construction without explanation",
			body: strings.Replace(validUnit, "        explanation: because", "", 1),
			want: "construction needs an explanation",
		},
		{
			name: "unknown kind",
			body: strings.Replace(validUnit, "kind: construction", "kind: flashcards", 1),
			want: `unknown kind "flashcards"`,
		},
		{
			name: "speaking with both grading paths",
			body: strings.Replace(validUnit, "kind: construction", "kind: speaking\n        rubric: judge it", 1),
			want: "speaking has both accept and rubric",
		},
		{
			name: "speaking with no grading path",
			body: strings.Replace(
				strings.Replace(validUnit, "kind: construction", "kind: speaking", 1),
				"        accept:\n          - kana: あ\n            romaji: a\n", "", 1),
			want: "speaking needs either accept answers or a rubric",
		},
		{
			name: "interpretation without accepted meaning",
			body: strings.Replace(
				strings.Replace(validUnit, "kind: construction", "kind: interpretation", 1),
				"        accept:\n          - kana: あ\n            romaji: a",
				"        target:\n          kana: あ\n          romaji: a", 1),
			want: "interpretation needs at least one accept_meaning",
		},
		{
			name: "listening without target",
			body: strings.Replace(validUnit, "kind: construction", "kind: listening", 1),
			want: "listening needs a target",
		},
		{
			name: "audio provenance incomplete",
			body: strings.Replace(validUnit, "        prompt: Say it.",
				"        audio: clips/a.wav\n        audio_source: Tatoeba\n        prompt: Say it.", 1),
			want: "audio provenance must include audio_license",
		},
		{
			name: "conversation without scenario",
			body: strings.Replace(validUnit, "        kind: construction", "        kind: conversation", 1),
			want: "conversation needs a scenario",
		},
		{
			name: "conversation turn with bad speaker",
			body: strings.Replace(validUnit, "        kind: construction",
				"        kind: conversation\n        scenario:\n          turns:\n            - speaker: narrator\n              line:\n                kana: あ\n                romaji: a", 1),
			want: `speaker must be "npc" or "learner"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, issues, err := Load(writeUnit(t, "u.yaml", tc.body))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !hasIssue(issues, tc.want) {
				t.Fatalf("expected issue containing %q, got %v", tc.want, issues)
			}
		})
	}
}

func TestLoadRejectsDuplicateUnitIDsAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(validUnit), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	_, issues, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	write("a.yaml")
	write("b.yaml")
	_, issues, err = Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !hasIssue(issues, `duplicate unit id "u"`) {
		t.Fatalf("expected duplicate unit id issue, got %v", issues)
	}
}

func TestLoadMissingPathIsHardError(t *testing.T) {
	if _, _, err := Load(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("expected an error for a missing path")
	}
}

func TestRepoContentLints(t *testing.T) {
	_, issues, err := Load(filepath.Join("..", "..", "content"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("repo content has issues:\n%v", issues)
	}
}

const listeningUnit = `
id: u
title: Unit
concepts:
  - id: c1
lessons:
  - id: l1
    title: Lesson
    steps:
      - id: s1
        kind: listening
        concepts: [c1]
        prompt: What is being said about the place?
        target:
          kanji: 駅の近くにコンビニがあります
          kana: えきのちかくにコンビニがあります
          romaji: eki no chikaku ni konbini ga arimasu
%s        explanation: に marks where the thing exists.
`

func TestListeningGradesByDictationOrMeaning(t *testing.T) {
	const accept = `        accept:
          - kana: えきのちかくにコンビニがあります
            romaji: eki no chikaku ni konbini ga arimasu
`
	const meaning = `        accept_meanings:
          - There is a convenience store near the station.
`

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "dictation", body: fmt.Sprintf(listeningUnit, accept)},
		{name: "meaning", body: fmt.Sprintf(listeningUnit, meaning)},
		{
			name: "neither",
			body: fmt.Sprintf(listeningUnit, ""),
			want: "listening needs accepted answers or accept_meanings",
		},
		{
			name: "both grading paths",
			body: fmt.Sprintf(listeningUnit, accept+meaning),
			want: "pick one grading path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, issues, err := Load(writeUnit(t, "u.yaml", tt.body))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if tt.want == "" {
				if len(issues) != 0 {
					t.Fatalf("expected no issues, got %v", issues)
				}
				return
			}
			if !hasIssue(issues, tt.want) {
				t.Fatalf("expected an issue containing %q, got %v", tt.want, issues)
			}
		})
	}
}

const taughtUnit = `
id: u
title: Unit
taught: true
placement: true
concepts:
  - id: c1
  - id: wa
lessons:
  - id: l1
    title: Lesson
    objective: Say where something exists.
    anchors:
      - kanji: ここはどこですか
        kana: ここはどこですか
        romaji: koko wa doko desu ka
    teach:
      - prose: に marks where a thing exists.
        examples:
          - kanji: 駅にあります
            kana: えきにあります
            romaji: eki ni arimasu
    steps:
      - id: s1
        kind: construction
        concepts: [c1]
        prompt: Say it.
        hint: Think about the particle.
        accept:
          - kana: すしはたべません
            romaji: sushi wa tabemasen
            concepts: [wa]
        explanation: because
`

func TestTaughtUnitDecodesLearnFields(t *testing.T) {
	units, issues, err := Load(writeUnit(t, "u.yaml", taughtUnit))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
	unit := units[0]
	if !unit.Taught || !unit.Placement {
		t.Fatalf("unit flags = taught:%v placement:%v", unit.Taught, unit.Placement)
	}
	lesson := unit.Lessons[0]
	if lesson.Objective != "Say where something exists." {
		t.Fatalf("objective = %q", lesson.Objective)
	}
	if len(lesson.Anchors) != 1 || lesson.Anchors[0].Romaji != "koko wa doko desu ka" {
		t.Fatalf("anchors = %+v", lesson.Anchors)
	}
	if len(lesson.Teach) != 1 || lesson.Teach[0].Prose != "に marks where a thing exists." {
		t.Fatalf("teach = %+v", lesson.Teach)
	}
	if len(lesson.Teach[0].Examples) != 1 || lesson.Teach[0].Examples[0].Kana != "えきにあります" {
		t.Fatalf("teach examples = %+v", lesson.Teach[0].Examples)
	}
	step := lesson.Steps[0]
	if step.Hint != "Think about the particle." {
		t.Fatalf("hint = %q", step.Hint)
	}
	if len(step.Accept) != 1 || len(step.Accept[0].Concepts) != 1 || step.Accept[0].Concepts[0] != "wa" {
		t.Fatalf("accept concepts = %+v", step.Accept)
	}
}

func TestTaughtUnitRequiresObjectiveAndTeach(t *testing.T) {
	body := strings.NewReplacer(
		"    objective: Say where something exists.\n", "",
		"    anchors:\n      - kanji: ここはどこですか\n        kana: ここはどこですか\n        romaji: koko wa doko desu ka\n", "",
		"    teach:\n      - prose: に marks where a thing exists.\n        examples:\n          - kanji: 駅にあります\n            kana: えきにあります\n            romaji: eki ni arimasu\n", "",
	).Replace(taughtUnit)
	_, issues, err := Load(writeUnit(t, "u.yaml", body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !hasIssue(issues, "lesson needs an objective") {
		t.Fatalf("expected a missing objective issue, got %v", issues)
	}
	if !hasIssue(issues, "lesson needs at least one teach entry") {
		t.Fatalf("expected a missing teach issue, got %v", issues)
	}
}

func TestLearnFieldsValidate(t *testing.T) {
	tests := []struct {
		name    string
		oldText string
		newText string
		want    string
	}{
		{
			name:    "teach entry without prose",
			oldText: "      - prose: に marks where a thing exists.\n        examples:\n",
			newText: "      - examples:\n",
			want:    "needs prose",
		},
		{
			name:    "teach example without kana",
			oldText: "            kana: えきにあります\n",
			newText: "",
			want:    "missing kana reading",
		},
		{
			name:    "anchor without romaji",
			oldText: "        romaji: koko wa doko desu ka\n",
			newText: "",
			want:    "missing authored romaji",
		},
		{
			name:    "accept concept undeclared",
			oldText: "            concepts: [wa]",
			newText: "            concepts: [nope]",
			want:    `undeclared concept "nope"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, issues, err := Load(writeUnit(t, "u.yaml", strings.Replace(taughtUnit, tc.oldText, tc.newText, 1)))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !hasIssue(issues, tc.want) {
				t.Fatalf("expected an issue containing %q, got %v", tc.want, issues)
			}
		})
	}
}

func hasIssue(issues []Issue, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue.String(), want) {
			return true
		}
	}
	return false
}
