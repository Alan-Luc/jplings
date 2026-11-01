package content

import (
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
	// Two files declaring the same unit id must be caught before the engine
	// sees two profiles' worth of progress pointing at one id.
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

// TestRepoContentLints is the build check D7 asks for: authored content must stay
// schema-valid, so content errors break the build rather than a session.
func TestRepoContentLints(t *testing.T) {
	_, issues, err := Load(filepath.Join("..", "..", "content"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("repo content has issues:\n%v", issues)
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
