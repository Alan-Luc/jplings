package content

import "fmt"

type Coverage struct {
	UnitID        string
	ConceptID     string
	Comprehension int
	Production    int
}

func (c Coverage) Complete() bool { return c.Comprehension >= 1 && c.Production >= 2 }

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

		for _, concept := range unit.Concepts {
			if _, ok := index[concept.ID]; !ok {
				out = append(out, Coverage{UnitID: unit.ID, ConceptID: concept.ID})
			}
		}
	}

	return out
}

func (c Coverage) SummaryLine() string {
	state := "ok"
	if !c.Complete() {
		state = "thin"
	}
	return fmt.Sprintf("%-28s comprehension=%-2d production=%-2d %s",
		c.ConceptID, c.Comprehension, c.Production, state)
}
