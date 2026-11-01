package grade

import "jplings/internal/content"

func MatchedConcepts(step *content.Step, answer string) []string {
	for i := range step.Accept {
		single := *step
		single.Accept = step.Accept[i : i+1]

		var outcome Outcome
		if step.Kind == content.Speaking {
			outcome = Spoken(&single, answer)
		} else {
			outcome = Written(&single, answer)
		}
		if outcome.Correct {
			return step.Accept[i].Concepts
		}
	}
	return nil
}
