package corpus

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type Sentence struct {
	Japanese string `yaml:"japanese"`
	English  string `yaml:"english"`
	ID       string `yaml:"tatoeba_id"`
}

// ReadSentences parses a Tatoeba per-language sentence export
// (id TAB language TAB text).
func ReadSentences(r io.Reader) (map[string]string, error) {
	sentences := map[string]string{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20)

	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 3)
		if len(fields) != 3 {
			continue
		}
		sentences[fields[0]] = fields[2]
	}
	return sentences, scanner.Err()
}

// ReadLinks parses a Tatoeba language-pair link export (jpn_id TAB eng_id).
func ReadLinks(r io.Reader) ([][2]string, error) {
	var links [][2]string
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20)

	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 2)
		if len(fields) != 2 {
			continue
		}
		links = append(links, [2]string{fields[0], fields[1]})
	}
	return links, scanner.Err()
}

func Pairs(japanese, english map[string]string, links [][2]string) []Sentence {
	var pairs []Sentence
	seen := map[string]bool{}

	for _, link := range links {
		jpn, jpnOK := japanese[link[0]]
		eng, engOK := english[link[1]]
		if !jpnOK || !engOK || strings.TrimSpace(jpn) == "" || strings.TrimSpace(eng) == "" {
			continue
		}
		if seen[link[0]] {
			continue
		}
		seen[link[0]] = true
		pairs = append(pairs, Sentence{Japanese: strings.TrimSpace(jpn), English: strings.TrimSpace(eng), ID: link[0]})
	}
	return pairs
}

// blockedTerms are excluded from imports. Tatoeba is a general corpus and its
// shortest sentences include insults; a language course must not hand those to a
// learner as practice material.
var blockedTerms = []string{
	"死ね", "死ぬ", "殺す", "殺し", "殺せ", "殺した",
	"クソ", "くそ", "クソッ", "うざい", "ウザい", "きもい", "キモい", "むかつく", "ムカつく",
	"セックス", "レイプ", "チンポ", "ちんこ", "まんこ", "おっぱい", "変態", "エロ",
	"ぶっ殺す", "消えろ", "うせろ",
}

// FilterOptions controls which imported sentences are usable as material.
//
// FilterOptions zero value keeps everything that is not blocked, which is the right
// default for a reference export; teaching material should also set MinRunes and
// Require so the sentences are substantial and drawn from curated vocabulary.
type FilterOptions struct {
	// MinRunes rejects one-word interjections, which teach nothing on their own.
	MinRunes int
	// Require keeps only sentences containing at least one of these forms, so the
	// material stays inside vocabulary the curriculum actually teaches.
	Require map[string]bool
}

func Filter(pairs []Sentence, options FilterOptions) []Sentence {
	var kept []Sentence
	for _, pair := range pairs {
		if !Usable(pair, options) {
			continue
		}
		kept = append(kept, pair)
	}
	return kept
}

func Usable(pair Sentence, options FilterOptions) bool {
	if options.MinRunes > 0 && len([]rune(pair.Japanese)) < options.MinRunes {
		return false
	}
	for _, term := range blockedTerms {
		if strings.Contains(pair.Japanese, term) {
			return false
		}
	}
	if len(options.Require) > 0 {
		for form := range options.Require {
			if strings.Contains(pair.Japanese, form) {
				return true
			}
		}
		return false
	}
	return true
}

// SelectShort returns the shortest usable pairs, then orders them by id for stable
// output. Short sentences are the right starting material for a beginner.
func SelectShort(pairs []Sentence, limit int) []Sentence {
	sorted := append([]Sentence(nil), pairs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := len([]rune(sorted[i].Japanese)), len([]rune(sorted[j].Japanese))
		if left != right {
			return left < right
		}
		return numericLess(sorted[i].ID, sorted[j].ID)
	})
	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}
	sort.SliceStable(sorted, func(i, j int) bool { return numericLess(sorted[i].ID, sorted[j].ID) })
	return sorted
}

func numericLess(a, b string) bool {
	left, leftErr := strconv.Atoi(a)
	right, rightErr := strconv.Atoi(b)
	if leftErr == nil && rightErr == nil {
		return left < right
	}
	return a < b
}

// Validate reports pairs that cannot be used as content, so a bad download fails
// loudly instead of producing empty exercises.
func Validate(pairs []Sentence) error {
	if len(pairs) == 0 {
		return fmt.Errorf("no sentence pairs found; check the language exports")
	}
	return nil
}
