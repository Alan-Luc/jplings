package content

import "testing"

func TestCoverageCountsDimensions(t *testing.T) {
	unit := &Unit{
		ID:       "u",
		Concepts: []Concept{{ID: "deep"}, {ID: "thin"}, {ID: "never"}},
		Lessons: []Lesson{{ID: "l", Title: "L", Steps: []Step{
			{ID: "s1", Kind: Interpretation, Concepts: []string{"deep"}},
			{ID: "s2", Kind: Construction, Concepts: []string{"deep"}},
			{ID: "s3", Kind: Transformation, Concepts: []string{"deep"}},
			{ID: "s4", Kind: Construction, Concepts: []string{"thin"}},
		}}},
	}

	got := map[string]Coverage{}
	for _, coverage := range CoverageOf([]*Unit{unit}) {
		got[coverage.ConceptID] = coverage
	}

	deep := got["deep"]
	if deep.Comprehension != 1 || deep.Production != 2 || !deep.Complete() {
		t.Fatalf("deep concept = %+v, want 1 comprehension and 2 production", deep)
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
