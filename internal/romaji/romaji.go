// Package romaji normalizes and compares learner answers (D7).
//
// Matching runs two passes but only one earns credit. The exact pass normalizes
// spelling conventions only — Hepburn versus wāpuro — and is the sole path to a
// correct verdict. The close pass additionally collapses vowel length and
// gemination; it never awards credit, because おばさん and おばあさん are different
// words, but it lets the app name the distinction instead of reporting a bare miss.
package romaji

import "strings"

type Verdict int

const (
	// Wrong differs in more than length or doubling.
	Wrong Verdict = iota
	// Close differs only in vowel length or gemination. No credit.
	Close
	// Exact matches an accepted answer under spelling conventions. Earns credit.
	Exact
)

// spelling folds Hepburn and wāpuro spellings of the same sound. Longest patterns
// come first because NewReplacer tries arguments in order at each position.
var spelling = strings.NewReplacer(
	"ā", "aa", "ī", "ii", "ū", "uu", "ē", "ee", "ō", "oo",
	"sha", "sha", "shu", "shu", "sho", "sho", "shi", "shi",
	"sya", "sha", "syu", "shu", "syo", "sho", "si", "shi",
	"tya", "cha", "tyu", "chu", "tyo", "cho", "ti", "chi",
	"tu", "tsu",
	"hu", "fu",
	"zi", "ji",
	"jya", "ja", "jyu", "ju", "jyo", "jo",
	"zya", "ja", "zyu", "ju", "zyo", "jo",
	"cya", "cha", "cyu", "chu", "cyo", "cho",
)

var (
	doubled = []string{"kk", "ss", "tt", "pp", "cc", "dd", "gg", "bb", "rr", "hh", "ff", "jj", "zz", "vv", "yy", "ww"}
	long    = []string{"aa", "ii", "uu", "ee", "oo", "ou", "ei", "nn"}
)

// NormalizeExact reduces an answer to its spelling-convention-free form. It never
// merges two different words: vowel length and gemination survive.
func NormalizeExact(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	// を is romanized both "o" (Hepburn) and "wo" (wāpuro). Only a standalone token
	// is folded, so こわい is never mangled into こあい.
	fields := strings.Fields(s)
	for i, field := range fields {
		if field == "wo" {
			fields[i] = "o"
		}
	}
	s = strings.Join(fields, " ")
	return spelling.Replace(s)
}

// NormalizeClose additionally collapses vowel length and gemination, in both romaji
// and kana. It is only ever used to explain a near miss; it never awards credit.
func NormalizeClose(s string) string {
	s = NormalizeExact(s)
	s = collapseGemination(s)
	return collapseVowelLength(s)
}

// collapseGemination removes doubled consonants and the kana sokuon, which is how
// Japanese writes the same feature in either script.
func collapseGemination(s string) string {
	s = strings.ReplaceAll(s, "っ", "")
	previous := ""
	for previous != s {
		previous = s
		for _, pair := range doubled {
			s = strings.ReplaceAll(s, pair, pair[:1])
		}
	}
	return s
}

// collapseVowelLength also strips the katakana chōonpu: ビール and ビル are different
// words, so this belongs in the close pass, never in the one that awards credit.
func collapseVowelLength(s string) string {
	s = strings.ReplaceAll(s, "ー", "")
	previous := ""
	for previous != s {
		previous = s
		for _, pair := range long {
			s = strings.ReplaceAll(s, pair, pair[:1])
		}
	}
	return s
}

func Compare(input string, accepted []string) (Verdict, string) {
	exact := NormalizeExact(input)
	for _, want := range accepted {
		if exact == NormalizeExact(want) {
			return Exact, ""
		}
	}

	close := NormalizeClose(input)
	for _, want := range accepted {
		if close == NormalizeClose(want) {
			return Close, distinction(input, want)
		}
	}
	return Wrong, ""
}

// distinction names the feature separating two close forms, so feedback teaches
// rather than just refusing. A feature only counts when collapsing it actually
// changed one of the two forms, otherwise every pair would trivially "agree".
func distinction(input, want string) string {
	a, b := NormalizeExact(input), NormalizeExact(want)

	gemination := collapseGemination(a) == collapseGemination(b) &&
		(collapseGemination(a) != a || collapseGemination(b) != b)
	vowels := collapseVowelLength(a) == collapseVowelLength(b) &&
		(collapseVowelLength(a) != a || collapseVowelLength(b) != b)

	switch {
	case gemination && vowels:
		return "vowel length and gemination"
	case gemination:
		return "gemination"
	case vowels:
		return "vowel length"
	default:
		return "spelling"
	}
}
