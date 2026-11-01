package content

import (
	"fmt"
	"testing"
)

func TestCoverageCountsDimensions(t *testing.T) {
	unit := &Unit{
		ID:       "u",
		Concepts: []Concept{{ID: "deep"}, {ID: "thin"}, {ID: "never"}},
		Lessons: []Lesson{{ID: "l", Title: "L", Steps: []Step{
			{ID: "s1", Kind: Interpretation, Concepts: []string{"deep"}},
			{ID: "s2", Kind: Construction, Concepts: []string{"deep"}},
			{ID: "s3", Kind: Transformation, Concepts: []string{"deep"}},
			{ID: "s4", Kind: Construction, Concepts: []string{"deep"}},
			{ID: "s5", Kind: Construction, Concepts: []string{"thin"}},
		}}},
	}

	got := map[string]Coverage{}
	for _, coverage := range CoverageOf([]*Unit{unit}) {
		got[coverage.ConceptID] = coverage
	}

	deep := got["deep"]
	if deep.Comprehension != 1 || deep.Production != 3 || !deep.Complete() {
		t.Fatalf("deep concept = %+v, want 1 comprehension and 3 production", deep)
	}

	thin := got["thin"]
	if thin.Complete() {
		t.Fatalf("a single production step must not count as complete: %+v", thin)
	}

	never, ok := got["never"]
	if !ok {
		t.Fatal("a declared but unexercised concept must still be reported")
	}
	if never.Complete() {
		t.Fatalf("an unexercised concept must not be complete: %+v", never)
	}
}

func TestCoverageFloorGatesTaughtUnitsOnly(t *testing.T) {
	unit := func(taught bool, production int) *Unit {
		steps := []Step{{
			ID: "s0", Kind: Interpretation, Concepts: []string{"c"},
			Prompt: "p", Target: &Text{Kana: "あ", Romaji: "a"},
			AcceptMeanings: []string{"a"}, Explanation: "e",
		}}
		for i := 0; i < production; i++ {
			steps = append(steps, Step{
				ID:          fmt.Sprintf("s%d", i+1),
				Kind:        Construction,
				Concepts:    []string{"c"},
				Prompt:      "p",
				Accept:      []Text{{Kana: "あ", Romaji: "a"}},
				Explanation: "e",
			})
		}
		return &Unit{
			ID: "u", Title: "U", Taught: taught,
			Concepts: []Concept{{ID: "c"}},
			Lessons: []Lesson{{
				ID: "l", Title: "L", Objective: "o",
				Teach: []Teach{{Prose: "p"}},
				Steps: steps,
			}},
		}
	}

	tests := []struct {
		name       string
		taught     bool
		production int
		wantIssue  bool
	}{
		{name: "taught below the floor", taught: true, production: 2, wantIssue: true},
		{name: "taught at the floor", taught: true, production: 3},
		{name: "untaught below the floor", taught: false, production: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issues := unit(tc.taught, tc.production).Validate()
			if got := hasIssue(issues, "below the coverage floor"); got != tc.wantIssue {
				t.Fatalf("floor issue = %t, want %t; issues: %v", got, tc.wantIssue, issues)
			}
		})
	}
}

func TestCoverageMapsEveryGradedKind(t *testing.T) {
	kinds := map[Kind]string{
		Interpretation: "comprehension",
		Listening:      "comprehension",
		Construction:   "production",
		Transformation: "production",
		Completion:     "production",
		Speaking:       "production",
		Conversation:   "production",
	}
	for kind, want := range kinds {
		if got := Dimension(kind); got != want {
			t.Fatalf("Dimension(%s) = %q, want %q", kind, got, want)
		}
	}
}
