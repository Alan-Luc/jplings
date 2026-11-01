package romaji

import "strings"

type Verdict int

const (
	Wrong Verdict = iota
	Close
	Exact
)

type syllable struct {
	romaji string
}

var syllables = map[string]syllable{
	"a": {"a"}, "i": {"i"}, "u": {"u"}, "e": {"e"}, "o": {"o"},

	"ka": {"ka"}, "ki": {"ki"}, "ku": {"ku"}, "ke": {"ke"}, "ko": {"ko"},
	"kya": {"kya"}, "kyu": {"kyu"}, "kyo": {"kyo"},

	"sa": {"sa"}, "shi": {"shi"}, "si": {"shi"}, "su": {"su"}, "se": {"se"}, "so": {"so"},
	"sha": {"sha"}, "sya": {"sha"}, "shu": {"shu"}, "syu": {"shu"},
	"sho": {"sho"}, "syo": {"sho"}, "she": {"she"},

	"ta": {"ta"}, "chi": {"chi"}, "ti": {"chi"}, "tsu": {"tsu"}, "tu": {"tsu"},
	"te": {"te"}, "to": {"to"},
	"cha": {"cha"}, "tya": {"cha"}, "cya": {"cha"},
	"chu": {"chu"}, "tyu": {"chu"}, "cyu": {"chu"},
	"cho": {"cho"}, "tyo": {"cho"}, "cyo": {"cho"}, "che": {"che"},
	"tsa": {"tsa"}, "tsi": {"tsi"}, "tse": {"tse"}, "tso": {"tso"},

	"na": {"na"}, "ni": {"ni"}, "nu": {"nu"}, "ne": {"ne"}, "no": {"no"},
	"nya": {"nya"}, "nyu": {"nyu"}, "nyo": {"nyo"}, "n": {"n"},

	"ha": {"ha"}, "hi": {"hi"}, "fu": {"fu"}, "hu": {"fu"}, "he": {"he"}, "ho": {"ho"},
	"hya": {"hya"}, "hyu": {"hyu"}, "hyo": {"hyo"},

	"ma": {"ma"}, "mi": {"mi"}, "mu": {"mu"}, "me": {"me"}, "mo": {"mo"},
	"mya": {"mya"}, "myu": {"myu"}, "myo": {"myo"},

	"ya": {"ya"}, "yu": {"yu"}, "yo": {"yo"},

	"ra": {"ra"}, "ri": {"ri"}, "ru": {"ru"}, "re": {"re"}, "ro": {"ro"},
	"rya": {"rya"}, "ryu": {"ryu"}, "ryo": {"ryo"},

	"wa": {"wa"}, "wi": {"wi"}, "we": {"we"}, "wo": {"wo"},

	"ga": {"ga"}, "gi": {"gi"}, "gu": {"gu"}, "ge": {"ge"}, "go": {"go"},
	"gya": {"gya"}, "gyu": {"gyu"}, "gyo": {"gyo"},

	"za": {"za"}, "ji": {"ji"}, "zi": {"ji"}, "zu": {"zu"}, "ze": {"ze"}, "zo": {"zo"},
	"ja": {"ja"}, "jya": {"ja"}, "zya": {"ja"},
	"ju": {"ju"}, "jyu": {"ju"}, "zyu": {"ju"},
	"jo": {"jo"}, "jyo": {"jo"}, "zyo": {"jo"}, "je": {"je"},

	"da": {"da"}, "di": {"di"}, "du": {"du"}, "de": {"de"}, "do": {"do"},
	"dyu": {"dyu"},

	"ba": {"ba"}, "bi": {"bi"}, "bu": {"bu"}, "be": {"be"}, "bo": {"bo"},
	"bya": {"bya"}, "byu": {"byu"}, "byo": {"byo"},

	"pa": {"pa"}, "pi": {"pi"}, "pu": {"pu"}, "pe": {"pe"}, "po": {"po"},
	"pya": {"pya"}, "pyu": {"pyu"}, "pyo": {"pyo"},

	"fa": {"fa"}, "fi": {"fi"}, "fe": {"fe"}, "fo": {"fo"},
	"va": {"va"}, "vi": {"vi"}, "vu": {"vu"}, "ve": {"ve"}, "vo": {"vo"},
}

var macrons = strings.NewReplacer("ā", "aa", "ī", "ii", "ū", "uu", "ē", "ee", "ō", "ou")

var (
	doubled = []string{"kk", "ss", "tt", "pp", "cc", "dd", "gg", "bb", "rr", "hh", "ff", "jj", "zz", "vv", "yy", "ww"}
	long    = []string{"aa", "ii", "uu", "ee", "oo", "ou", "ei", "nn"}
)

func NormalizeExact(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	fields := strings.Fields(s)
	for i, field := range fields {
		if field == "wo" {
			fields[i] = "o"
		}
	}
	// Spacing is not phonemic, and imports emit per-morpheme romaji ("nomi masu")
	// while authored answers are often unspaced ("nomimasu"), so both sides drop it.
	s = strings.Join(fields, "")
	return normalizeRomaji(s)
}

func normalizeRomaji(s string) string {
	runes := []rune(macrons.Replace(s))
	var out strings.Builder

	for i := 0; i < len(runes); {
		if runes[i] == 'n' && i+1 < len(runes) && runes[i+1] == 'n' {
			out.WriteRune('n')
			i++
			continue
		}
		if i+1 < len(runes) && isConsonant(runes[i]) && runes[i] == runes[i+1] {
			out.WriteRune(runes[i])
			i++
			continue
		}
		if key, next, ok := match(runes[i:]); ok {
			out.WriteString(next.romaji)
			i += len([]rune(key))
			continue
		}
		out.WriteRune(runes[i])
		i++
	}
	return out.String()
}

func match(runes []rune) (string, syllable, bool) {
	for length := 3; length >= 1; length-- {
		if length > len(runes) {
			continue
		}
		key := string(runes[:length])
		if next, ok := syllables[key]; ok {
			return key, next, true
		}
	}
	return "", syllable{}, false
}

func isConsonant(r rune) bool {
	return r >= 'a' && r <= 'z' && !strings.ContainsRune("aiueo", r)
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
