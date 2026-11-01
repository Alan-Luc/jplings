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
	kana   string
}

var syllables = map[string]syllable{
	"a": {"a", "あ"}, "i": {"i", "い"}, "u": {"u", "う"}, "e": {"e", "え"}, "o": {"o", "お"},

	"ka": {"ka", "か"}, "ki": {"ki", "き"}, "ku": {"ku", "く"}, "ke": {"ke", "け"}, "ko": {"ko", "こ"},
	"kya": {"kya", "きゃ"}, "kyu": {"kyu", "きゅ"}, "kyo": {"kyo", "きょ"},

	"sa": {"sa", "さ"}, "shi": {"shi", "し"}, "si": {"shi", "し"}, "su": {"su", "す"}, "se": {"se", "せ"}, "so": {"so", "そ"},
	"sha": {"sha", "しゃ"}, "sya": {"sha", "しゃ"}, "shu": {"shu", "しゅ"}, "syu": {"shu", "しゅ"},
	"sho": {"sho", "しょ"}, "syo": {"sho", "しょ"}, "she": {"she", "しぇ"},

	"ta": {"ta", "た"}, "chi": {"chi", "ち"}, "ti": {"chi", "ち"}, "tsu": {"tsu", "つ"}, "tu": {"tsu", "つ"},
	"te": {"te", "て"}, "to": {"to", "と"},
	"cha": {"cha", "ちゃ"}, "tya": {"cha", "ちゃ"}, "cya": {"cha", "ちゃ"},
	"chu": {"chu", "ちゅ"}, "tyu": {"chu", "ちゅ"}, "cyu": {"chu", "ちゅ"},
	"cho": {"cho", "ちょ"}, "tyo": {"cho", "ちょ"}, "cyo": {"cho", "ちょ"}, "che": {"che", "ちぇ"},
	"tsa": {"tsa", "つぁ"}, "tsi": {"tsi", "つぃ"}, "tse": {"tse", "つぇ"}, "tso": {"tso", "つぉ"},

	"na": {"na", "な"}, "ni": {"ni", "に"}, "nu": {"nu", "ぬ"}, "ne": {"ne", "ね"}, "no": {"no", "の"},
	"nya": {"nya", "にゃ"}, "nyu": {"nyu", "にゅ"}, "nyo": {"nyo", "にょ"}, "n": {"n", "ん"},

	"ha": {"ha", "は"}, "hi": {"hi", "ひ"}, "fu": {"fu", "ふ"}, "hu": {"fu", "ふ"}, "he": {"he", "へ"}, "ho": {"ho", "ほ"},
	"hya": {"hya", "ひゃ"}, "hyu": {"hyu", "ひゅ"}, "hyo": {"hyo", "ひょ"},

	"ma": {"ma", "ま"}, "mi": {"mi", "み"}, "mu": {"mu", "む"}, "me": {"me", "め"}, "mo": {"mo", "も"},
	"mya": {"mya", "みゃ"}, "myu": {"myu", "みゅ"}, "myo": {"myo", "みょ"},

	"ya": {"ya", "や"}, "yu": {"yu", "ゆ"}, "yo": {"yo", "よ"},

	"ra": {"ra", "ら"}, "ri": {"ri", "り"}, "ru": {"ru", "る"}, "re": {"re", "れ"}, "ro": {"ro", "ろ"},
	"rya": {"rya", "りゃ"}, "ryu": {"ryu", "りゅ"}, "ryo": {"ryo", "りょ"},

	"wa": {"wa", "わ"}, "wi": {"wi", "うぃ"}, "we": {"we", "うぇ"}, "wo": {"wo", "を"},

	"ga": {"ga", "が"}, "gi": {"gi", "ぎ"}, "gu": {"gu", "ぐ"}, "ge": {"ge", "げ"}, "go": {"go", "ご"},
	"gya": {"gya", "ぎゃ"}, "gyu": {"gyu", "ぎゅ"}, "gyo": {"gyo", "ぎょ"},

	"za": {"za", "ざ"}, "ji": {"ji", "じ"}, "zi": {"ji", "じ"}, "zu": {"zu", "ず"}, "ze": {"ze", "ぜ"}, "zo": {"zo", "ぞ"},
	"ja": {"ja", "じゃ"}, "jya": {"ja", "じゃ"}, "zya": {"ja", "じゃ"},
	"ju": {"ju", "じゅ"}, "jyu": {"ju", "じゅ"}, "zyu": {"ju", "じゅ"},
	"jo": {"jo", "じょ"}, "jyo": {"jo", "じょ"}, "zyo": {"jo", "じょ"}, "je": {"je", "じぇ"},

	"da": {"da", "だ"}, "di": {"di", "ぢ"}, "du": {"du", "づ"}, "de": {"de", "で"}, "do": {"do", "ど"},
	"dyu": {"dyu", "でゅ"},

	"ba": {"ba", "ば"}, "bi": {"bi", "び"}, "bu": {"bu", "ぶ"}, "be": {"be", "べ"}, "bo": {"bo", "ぼ"},
	"bya": {"bya", "びゃ"}, "byu": {"byu", "びゅ"}, "byo": {"byo", "びょ"},

	"pa": {"pa", "ぱ"}, "pi": {"pi", "ぴ"}, "pu": {"pu", "ぷ"}, "pe": {"pe", "ぺ"}, "po": {"po", "ぽ"},
	"pya": {"pya", "ぴゃ"}, "pyu": {"pyu", "ぴゅ"}, "pyo": {"pyo", "ぴょ"},

	"fa": {"fa", "ふぁ"}, "fi": {"fi", "ふぃ"}, "fe": {"fe", "ふぇ"}, "fo": {"fo", "ふぉ"},
	"va": {"va", "ゔぁ"}, "vi": {"vi", "ゔぃ"}, "vu": {"vu", "ゔ"}, "ve": {"ve", "ゔぇ"}, "vo": {"vo", "ゔぉ"},
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

func ToKana(s string) string {
	runes := []rune(macrons.Replace(strings.ToLower(strings.TrimSpace(s))))
	var out strings.Builder

	for i := 0; i < len(runes); {
		if runes[i] == 'n' && i+1 < len(runes) && runes[i+1] == 'n' {
			out.WriteRune('ん')
			i++
			continue
		}
		if i+1 < len(runes) && isConsonant(runes[i]) && runes[i] == runes[i+1] {
			out.WriteRune('っ')
			i++
			continue
		}
		if key, next, ok := match(runes[i:]); ok {
			out.WriteString(next.kana)
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
