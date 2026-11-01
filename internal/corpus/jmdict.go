package corpus

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type Word struct {
	Kanji     []string `yaml:"kanji,omitempty"`
	Kana      []string `yaml:"kana"`
	Pos       []string `yaml:"pos,omitempty"`
	Glosses   []string `yaml:"glosses"`
	Frequency int      `yaml:"frequency,omitempty"`
	Seq       string   `yaml:"seq"`
}

type Lookup map[string]*Word

func LoadJMdict(r io.Reader, want map[string]bool) (Lookup, error) {
	decoder := xml.NewDecoder(r)
	decoder.Strict = false

	lookup := Lookup{}
	var current *Word
	var inEntry bool

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("jmdict: %w", err)
		}

		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "entry":
				inEntry = true
				current = &Word{}
			case "ent_seq":
				if inEntry && current != nil {
					var seq string
					if err := decoder.DecodeElement(&seq, &element); err != nil {
						return nil, err
					}
					current.Seq = strings.TrimSpace(seq)
				}
			case "keb":
				if inEntry && current != nil {
					var form string
					if err := decoder.DecodeElement(&form, &element); err != nil {
						return nil, err
					}
					current.Kanji = append(current.Kanji, strings.TrimSpace(form))
				}
			case "reb":
				if inEntry && current != nil {
					var form string
					if err := decoder.DecodeElement(&form, &element); err != nil {
						return nil, err
					}
					current.Kana = append(current.Kana, strings.TrimSpace(form))
				}
			case "pos":
				if inEntry && current != nil {
					var pos string
					if err := decoder.DecodeElement(&pos, &element); err != nil {
						return nil, err
					}
					if tag := posTag(strings.TrimSpace(pos)); tag != "" {
						current.addPos(tag)
					}
				}
			case "ke_pri", "re_pri":
				if inEntry && current != nil {
					var priority string
					if err := decoder.DecodeElement(&priority, &element); err != nil {
						return nil, err
					}
					current.addFrequency(priorityBand(strings.TrimSpace(priority)))
				}
			case "gloss":
				if inEntry && current != nil {
					var gloss string
					if err := decoder.DecodeElement(&gloss, &element); err != nil {
						return nil, err
					}
					gloss = strings.TrimSpace(gloss)
					if gloss != "" && len(current.Glosses) < 4 {
						current.Glosses = append(current.Glosses, gloss)
					}
				}
			}
		case xml.EndElement:
			if element.Name.Local == "entry" && current != nil {
				inEntry = false
				keep(current, want, lookup)
				current = nil
			}
		}
	}
	return lookup, nil
}

func keep(word *Word, want map[string]bool, lookup Lookup) {
	if len(word.Kana) == 0 || len(word.Glosses) == 0 {
		return
	}
	matched := false
	for _, form := range append(append([]string{}, word.Kanji...), word.Kana...) {
		if want[form] {
			matched = true
			break
		}
	}
	if !matched {
		return
	}
	for _, form := range append(append([]string{}, word.Kanji...), word.Kana...) {
		if _, exists := lookup[form]; !exists {
			lookup[form] = word
		}
	}
}

// priorityBand turns JMdict priority tags into a frequency band: lower is more
// frequent, and zero means JMdict records no frequency for the word. nfXX is the
// news-corpus band 1-48; the other tags are coarser signals, and the lowest wins.
func priorityBand(tag string) int {
	if rest, ok := strings.CutPrefix(tag, "nf"); ok {
		if band, err := strconv.Atoi(rest); err == nil && band > 0 {
			return band
		}
	}
	switch tag {
	case "news1", "ichi1":
		return 1
	case "news2", "ichi2":
		return 2
	case "spec1":
		return 3
	case "spec2":
		return 4
	case "gai1":
		return 5
	case "gai2":
		return 6
	}
	return 0
}

func (w *Word) addFrequency(band int) {
	if band > 0 && (w.Frequency == 0 || band < w.Frequency) {
		w.Frequency = band
	}
}

func (w *Word) addPos(tag string) {
	for _, existing := range w.Pos {
		if existing == tag {
			return
		}
	}
	w.Pos = append(w.Pos, tag)
}

func posTag(raw string) string {
	raw = strings.TrimPrefix(raw, "&")
	raw = strings.TrimSuffix(raw, ";")
	return raw
}

// ResolveWords loads the requested words and fails when JMdict does not cover the
// whole list, so a truncated import cannot quietly write a short corpus.
func ResolveWords(r io.Reader, want map[string]bool) (Lookup, []*Word, error) {
	lookup, err := LoadJMdict(r, want)
	if err != nil {
		return nil, nil, err
	}
	words := lookup.Words()
	missing := MissingForms(want, words)
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("jmdict: resolved %d of %d requested words; missing: %s",
			len(want)-len(missing), len(want), strings.Join(missing, ", "))
	}
	return lookup, words, nil
}

func MissingForms(want map[string]bool, words []*Word) []string {
	matched := map[string]bool{}
	for _, word := range words {
		for _, form := range append(append([]string{}, word.Kanji...), word.Kana...) {
			if want[form] {
				matched[form] = true
			}
		}
	}
	var missing []string
	for form := range want {
		if !matched[form] {
			missing = append(missing, form)
		}
	}
	sort.Strings(missing)
	return missing
}

func (l Lookup) Words() []*Word {
	seen := map[*Word]bool{}
	var words []*Word
	for _, word := range l {
		if seen[word] {
			continue
		}
		seen[word] = true
		words = append(words, word)
	}
	sort.Slice(words, func(i, j int) bool {
		if words[i].Kana[0] != words[j].Kana[0] {
			return words[i].Kana[0] < words[j].Kana[0]
		}
		return words[i].Seq < words[j].Seq
	})
	return words
}
