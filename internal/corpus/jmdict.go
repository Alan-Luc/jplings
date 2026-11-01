package corpus

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Word struct {
	Kanji   []string `yaml:"kanji,omitempty"`
	Kana    []string `yaml:"kana"`
	Pos     []string `yaml:"pos,omitempty"`
	Glosses []string `yaml:"glosses"`
	Seq     string   `yaml:"seq"`
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
