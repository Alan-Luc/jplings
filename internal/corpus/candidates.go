package corpus

import (
	"io"
	"sort"
	"strings"
	"unicode"
)

// Candidate is a sentence scored for authoring. Coverage is the fraction of the
// sentence's non-punctuation characters covered by known words; Band is the
// highest (least frequent) JMdict band among those words, with zero meaning no
// frequency data.
type Candidate struct {
	Sentence `yaml:",inline"`
	Coverage float64  `yaml:"coverage"`
	Band     int      `yaml:"band"`
	Matched  []string `yaml:"matched_words,omitempty"`
}

type CandidatesFile struct {
	Corpus      string      `yaml:"corpus"`
	License     string      `yaml:"license"`
	Attribution string      `yaml:"attribution"`
	Source      string      `yaml:"source_url"`
	Candidates  []Candidate `yaml:"candidates"`
}

func WriteCandidates(w io.Writer, candidates []Candidate) error {
	return encode(w, CandidatesFile{
		Corpus:      JMdictName + " + " + TatoebaName,
		License:     JMdictLic + "; " + TatoebaLic,
		Attribution: JMdictAttr + "; " + TatoebaAttr,
		Source:      TatoebaJpn,
		Candidates:  candidates,
	})
}

// RankCandidates scores sentences by known-word coverage, then frequency band,
// then length and Tatoeba id. Sentences already present in existing content, or
// duplicated by another candidate, are dropped.
func RankCandidates(pairs []Sentence, known Lookup, existing []string, limit int) []Candidate {
	used := make(map[string]bool, len(existing))
	for _, text := range existing {
		used[dedupeKey(text)] = true
	}

	candidates := make([]Candidate, 0, len(pairs))
	for _, pair := range pairs {
		key := dedupeKey(pair.Japanese)
		if key == "" || used[key] {
			continue
		}
		used[key] = true
		coverage, band, matched := score(pair.Japanese, known)
		candidates = append(candidates, Candidate{
			Sentence: pair,
			Coverage: coverage,
			Band:     band,
			Matched:  matched,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.Coverage != right.Coverage {
			return left.Coverage > right.Coverage
		}
		if left.Band != right.Band {
			if left.Band == 0 {
				return false
			}
			if right.Band == 0 {
				return true
			}
			return left.Band < right.Band
		}
		if length := len([]rune(left.Japanese)); length != len([]rune(right.Japanese)) {
			return length < len([]rune(right.Japanese))
		}
		return numericLess(left.ID, right.ID)
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}

func score(text string, known Lookup) (float64, int, []string) {
	runes := []rune(text)
	covered := make([]bool, len(runes))
	total := 0
	for i, char := range runes {
		if !unicode.IsSpace(char) && !unicode.IsPunct(char) {
			covered[i] = true
			total++
		}
	}
	if total == 0 {
		return 0, 0, nil
	}

	type form struct {
		text  string
		runes []rune
		word  *Word
	}
	forms := make([]form, 0, len(known))
	for text, word := range known {
		forms = append(forms, form{text: text, runes: []rune(text), word: word})
	}
	sort.Slice(forms, func(i, j int) bool {
		if len(forms[i].runes) != len(forms[j].runes) {
			return len(forms[i].runes) > len(forms[j].runes)
		}
		return forms[i].text < forms[j].text
	})

	matched := map[*Word]bool{}
	matchedForms := map[string]bool{}
	for _, form := range forms {
		if len(form.runes) == 0 {
			continue
		}
		for start := 0; start+len(form.runes) <= len(runes); start++ {
			if !equalRunes(runes[start:start+len(form.runes)], form.runes) {
				continue
			}
			fresh := false
			for i := start; i < start+len(form.runes); i++ {
				if covered[i] {
					fresh = true
					covered[i] = false
				}
			}
			if fresh {
				matched[form.word] = true
				matchedForms[form.text] = true
			}
		}
	}

	band := 0
	for word := range matched {
		if word.Frequency == 0 {
			band = 0
			break
		}
		if word.Frequency > band {
			band = word.Frequency
		}
	}

	matchedList := make([]string, 0, len(matchedForms))
	for text := range matchedForms {
		matchedList = append(matchedList, text)
	}
	sort.Strings(matchedList)

	leftover := 0
	for _, unmatched := range covered {
		if unmatched {
			leftover++
		}
	}
	return float64(total-leftover) / float64(total), band, matchedList
}

func equalRunes(left, right []rune) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func dedupeKey(text string) string {
	return strings.Map(func(char rune) rune {
		if unicode.IsSpace(char) || unicode.IsPunct(char) {
			return -1
		}
		return char
	}, text)
}
