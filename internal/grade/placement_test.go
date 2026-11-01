package grade

import (
	"path/filepath"
	"testing"

	"jplings/internal/content"
)

func placementSteps(t *testing.T) map[string]*content.Step {
	t.Helper()

	units, issues, err := content.Load(filepath.Join("..", "..", "content"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("content issues: %v", issues)
	}

	steps := map[string]*content.Step{}
	for _, unit := range units {
		if unit.ID != "placement" {
			continue
		}
		for li := range unit.Lessons {
			for si := range unit.Lessons[li].Steps {
				step := &unit.Lessons[li].Steps[si]
				steps[step.ID] = step
			}
		}
	}
	if len(steps) == 0 {
		t.Fatal("no placement steps loaded")
	}
	return steps
}

func TestPlacementItemsGradeAsIntended(t *testing.T) {
	steps := placementSteps(t)

	tests := []struct {
		id      string
		answer  string
		correct bool
	}{
		{"p-rec-1", "Tanaka is a student.", true},
		{"p-rec-1", "Tanaka is a teacher.", false},
		{"p-rec-2", "there is a convenience store near the station", true},
		{"p-rec-3", "There's a cat in my room.", true},
		{"p-rec-4", "I will go to school tomorrow.", true},
		{"p-rec-5", "It is 800 yen.", true},
		{"p-rec-6", "What is this?", true},
		{"p-rec-7", "I have already eaten lunch.", true},
		{"p-rec-8", "Please wash your hands.", true},

		{"p-prod-1", "sushi o tabemashita", true},
		{"p-prod-1", "sushi wo tabemashita", true},
		{"p-prod-1", "sushi o tabemasu", false},
		{"p-prod-2", "toshokan de benkyou shimasu", true},
		{"p-prod-3", "heya ni neko ga imasu", true},
		{"p-prod-4", "sore wa watashi no kaban desu", true},
		{"p-prod-5", "nomanai", true},
		{"p-prod-5", "nomenai", false},
		{"p-prod-6", "ikimashita", true},
		{"p-prod-6", "itta", false},
		{"p-prod-7", "kaite", true},
		{"p-prod-8", "hassen en desu", true},
		{"p-prod-8", "hassen en", true},
		{"p-prod-8", "hachisen en", false},
	}

	for _, tc := range tests {
		step, ok := steps[tc.id]
		if !ok {
			t.Fatalf("step %s is missing from content/placement.yaml", tc.id)
		}

		outcome := Written(step, tc.answer)
		if len(step.AcceptMeanings) > 0 {
			outcome = Meaning(step, tc.answer)
		}

		if outcome.Correct != tc.correct {
			t.Fatalf("%s graded %q as correct=%t, want %t (%q)",
				tc.id, tc.answer, outcome.Correct, tc.correct, outcome.Explanation)
		}
		if outcome.Explanation == "" {
			t.Fatalf("%s graded %q with no explanation", tc.id, tc.answer)
		}
	}
}
