package content

import "fmt"

// Coverage says, for one concept, how many steps exercise it as comprehension and
// how many as production. A concept that is only ever read about is not learnable,
// and one that is produced only once cannot show transfer, so both counts matter.
type Coverage struct {
	UnitID        string
	ConceptID     string
	Comprehension int
	Production    int
}

// Complete reports whether a concept has the minimum evidence surface a unit must
// give it: at least one comprehension step and at least two production steps.
func (c Coverage) Complete() bool { return c.Comprehension >= 1 && c.Production >= 2 }

// Dimension maps an exercise kind to the evidence dimension it can demonstrate.
func Dimension(kind Kind) string {
	switch kind {
	case Interpretation, Listening:
		return "comprehension"
	case Construction, Transformation, Completion, Speaking, Conversation:
		return "production"
	default:
		return ""
	}
}

// CoverageOf counts, per unit and concept, how each concept is exercised.
func CoverageOf(units []*Unit) []Coverage {
	var out []Coverage

	for _, unit := range units {
		index := map[string]*Coverage{}
		var order []string

		for _, lesson := range unit.Lessons {
			for i := range lesson.Steps {
				step := &lesson.Steps[i]
				dimension := Dimension(step.Kind)
				if dimension == "" {
					continue
				}
				for _, conceptID := range step.Concepts {
					entry, ok := index[conceptID]
					if !ok {
						entry = &Coverage{UnitID: unit.ID, ConceptID: conceptID}
						index[conceptID] = entry
						order = append(order, conceptID)
					}
					if dimension == "comprehension" {
						entry.Comprehension++
					} else {
						entry.Production++
					}
				}
			}
		}

		for _, conceptID := range order {
			out = append(out, *index[conceptID])
		}

		// Declared but never exercised is the worst case: the unit promises to teach
		// something it never tests.
		for _, concept := range unit.Concepts {
			if _, ok := index[concept.ID]; !ok {
				out = append(out, Coverage{UnitID: unit.ID, ConceptID: concept.ID})
			}
		}
	}

	return out
}

// SummaryLine renders one coverage row for a report.
func (c Coverage) SummaryLine() string {
	state := "ok"
	if !c.Complete() {
		state = "thin"
	}
	return fmt.Sprintf("%-28s comprehension=%-2d production=%-2d %s",
		c.ConceptID, c.Comprehension, c.Production, state)
}
