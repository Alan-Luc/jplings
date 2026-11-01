package romaji

import "strings"

type Verdict int

const (
	Wrong Verdict = iota
	Close
	Exact
)

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

func NormalizeExact(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	fields := strings.Fields(s)
	for i, field := range fields {
		if field == "wo" {
			fields[i] = "o"
		}
	}
	s = strings.Join(fields, " ")
	return spelling.Replace(s)
}

func NormalizeClose(s string) string {
	s = NormalizeExact(s)
	s = collapseGemination(s)
	return collapseVowelLength(s)
}

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

// Stripping the katakana chōonpu belongs only here: ビール and ビル are different words, so the credit-earning pass must not do it.
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
