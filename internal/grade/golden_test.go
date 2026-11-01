package grade

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"jplings/internal/content"
)

type goldenEntry struct {
	Step     string   `yaml:"step"`
	Answer   string   `yaml:"answer"`
	Verdict  string   `yaml:"verdict"`
	Concepts []string `yaml:"concepts,omitempty"`
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..")
}

func loadContent(t *testing.T) map[string]*content.Step {
	t.Helper()

	units, issues, err := content.Load(filepath.Join(repoRoot(t), "content"))
	if err != nil {
		t.Fatalf("Load content: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("content issues: %v", issues)
	}

	steps := map[string]*content.Step{}
	for _, unit := range units {
		for li := range unit.Lessons {
			for si := range unit.Lessons[li].Steps {
				step := &unit.Lessons[li].Steps[si]
				steps[step.ID] = step
			}
		}
	}
	return steps
}

func loadGolden(t *testing.T) []goldenEntry {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "golden", "answers.yaml"))
	if err != nil {
		t.Fatalf("read golden answers: %v", err)
	}
	var entries []goldenEntry
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("parse golden answers: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("golden answers are empty")
	}
	return entries
}

func gradedConcepts(step *content.Step, answer string, correct bool) []string {
	if !correct {
		return nil
	}
	if matched := MatchedConcepts(step, answer); len(matched) > 0 {
		return matched
	}
	return step.Concepts
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, value := range got {
		seen[value]++
	}
	for _, value := range want {
		seen[value]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}

func TestGoldenAnswers(t *testing.T) {
	steps := loadContent(t)

	for _, entry := range loadGolden(t) {
		step, ok := steps[entry.Step]
		if !ok {
			t.Fatalf("golden entry names %q, which no longer exists in content/", entry.Step)
		}

		wantCorrect := entry.Verdict == "correct"
		switch entry.Verdict {
		case "correct", "wrong":
		default:
			t.Fatalf("%s: verdict %q must be \"correct\" or \"wrong\"", entry.Step, entry.Verdict)
		}

		outcome := Written(step, entry.Answer)
		switch {
		case step.Kind == content.Speaking:
			outcome = Spoken(step, entry.Answer)
		case len(step.AcceptMeanings) > 0:
			outcome = Meaning(step, entry.Answer)
		}

		if outcome.Correct != wantCorrect {
			t.Fatalf("%s graded %q as correct=%t, want %t (%q)",
				entry.Step, entry.Answer, outcome.Correct, wantCorrect, outcome.Explanation)
		}

		if len(entry.Concepts) > 0 {
			got := gradedConcepts(step, entry.Answer, outcome.Correct)
			if !sameSet(got, entry.Concepts) {
				t.Fatalf("%s graded %q crediting %v, want %v", entry.Step, entry.Answer, got, entry.Concepts)
			}
		}
	}
}

func TestEveryGradedStepHasAGoldenEntry(t *testing.T) {
	steps := loadContent(t)

	covered := map[string]bool{}
	for _, entry := range loadGolden(t) {
		covered[entry.Step] = true
	}

	for id, step := range steps {
		graded := len(step.Accept) > 0 || len(step.AcceptMeanings) > 0
		if graded && !covered[id] {
			t.Fatalf("step %q is graded but has no golden entry; a new accept without a golden answer is unchecked", id)
		}
	}
}
