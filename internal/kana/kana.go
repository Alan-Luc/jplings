package kana

import "strings"

var base = map[rune]string{
	'ア': "a", 'イ': "i", 'ウ': "u", 'エ': "e", 'オ': "o",
	'カ': "ka", 'キ': "ki", 'ク': "ku", 'ケ': "ke", 'コ': "ko",
	'サ': "sa", 'シ': "shi", 'ス': "su", 'セ': "se", 'ソ': "so",
	'タ': "ta", 'チ': "chi", 'ツ': "tsu", 'テ': "te", 'ト': "to",
	'ナ': "na", 'ニ': "ni", 'ヌ': "nu", 'ネ': "ne", 'ノ': "no",
	'ハ': "ha", 'ヒ': "hi", 'フ': "fu", 'ヘ': "he", 'ホ': "ho",
	'マ': "ma", 'ミ': "mi", 'ム': "mu", 'メ': "me", 'モ': "mo",
	'ヤ': "ya", 'ユ': "yu", 'ヨ': "yo",
	'ラ': "ra", 'リ': "ri", 'ル': "ru", 'レ': "re", 'ロ': "ro",
	'ワ': "wa", 'ヰ': "i", 'ヱ': "e", 'ヲ': "o",
	'ン': "n",
	'ガ': "ga", 'ギ': "gi", 'グ': "gu", 'ゲ': "ge", 'ゴ': "go",
	'ザ': "za", 'ジ': "ji", 'ズ': "zu", 'ゼ': "ze", 'ゾ': "zo",
	'ダ': "da", 'ヂ': "ji", 'ヅ': "zu", 'デ': "de", 'ド': "do",
	'バ': "ba", 'ビ': "bi", 'ブ': "bu", 'ベ': "be", 'ボ': "bo",
	'パ': "pa", 'ピ': "pi", 'プ': "pu", 'ペ': "pe", 'ポ': "po",
	'ヴ': "vu",
	'ァ': "a", 'ィ': "i", 'ゥ': "u", 'ェ': "e", 'ォ': "o",
	'ャ': "ya", 'ュ': "yu", 'ョ': "yo", 'ヮ': "wa",
}

var digraphs = map[string]string{
	"キャ": "kya", "キュ": "kyu", "キョ": "kyo",
	"シャ": "sha", "シュ": "shu", "ショ": "sho", "シェ": "she",
	"チャ": "cha", "チュ": "chu", "チョ": "cho", "チェ": "che",
	"ニャ": "nya", "ニュ": "nyu", "ニョ": "nyo",
	"ヒャ": "hya", "ヒュ": "hyu", "ヒョ": "hyo",
	"ミャ": "mya", "ミュ": "myu", "ミョ": "myo",
	"リャ": "rya", "リュ": "ryu", "リョ": "ryo",
	"ギャ": "gya", "ギュ": "gyu", "ギョ": "gyo",
	"ジャ": "ja", "ジュ": "ju", "ジョ": "jo", "ジェ": "je",
	"ビャ": "bya", "ビュ": "byu", "ビョ": "byo",
	"ピャ": "pya", "ピュ": "pyu", "ピョ": "pyo",
	"ティ": "ti", "ディ": "di", "デュ": "dyu",
	"ファ": "fa", "フィ": "fi", "フェ": "fe", "フォ": "fo",
	"ウィ": "wi", "ウェ": "we", "ウォ": "wo",
	"ヴァ": "va", "ヴィ": "vi", "ヴェ": "ve", "ヴォ": "vo",
	"ツァ": "tsa", "ツィ": "tsi", "ツェ": "tse", "ツォ": "tso",
}

var vowels = "aiueo"

// ToHiragana converts katakana to hiragana, leaving other runes untouched.
func ToHiragana(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 0x30A1 && r <= 0x30F6 {
			return r - 0x60
		}
		return r
	}, s)
}

// ToRomaji renders katakana (with small tsu and long-vowel marks) as Hepburn romaji.
// Callers pass pronunciation, not spelling, so particles already read as they sound.
func ToRomaji(s string) string {
	runes := []rune(s)
	var out strings.Builder

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if r == 'ッ' {
			if next := peekRomaji(runes, i+1); next != "" {
				out.WriteString(next[:1])
			}
			continue
		}
		if r == 'ー' {
			current := out.String()
			if current != "" {
				last := current[len(current)-1]
				if strings.ContainsRune(vowels, rune(last)) {
					out.WriteByte(last)
				}
			}
			continue
		}
		if i+1 < len(runes) {
			if pair, ok := digraphs[string(runes[i:i+2])]; ok {
				out.WriteString(pair)
				i++
				continue
			}
		}
		if romaji, ok := base[r]; ok {
			out.WriteString(romaji)
			continue
		}
	}
	return out.String()
}

func peekRomaji(runes []rune, index int) string {
	if index >= len(runes) {
		return ""
	}
	if index+1 < len(runes) {
		if pair, ok := digraphs[string(runes[index:index+2])]; ok {
			return pair
		}
	}
	return base[runes[index]]
}
