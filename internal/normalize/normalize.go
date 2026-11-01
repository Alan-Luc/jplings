package normalize

import (
	"strings"
	"unicode"

	"jplings/internal/romaji"
)

// Surface folds spelling only: it never converts kana, so particle presence survives.
func Surface(s string) string {
	s = strings.ToLower(foldFullwidth(strings.TrimSpace(s)))
	var stripped strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			stripped.WriteRune(' ')
		case unicode.IsPunct(r):
			continue
		default:
			stripped.WriteRune(r)
		}
	}

	fields := strings.Fields(digitsToKanji(stripped.String()))
	for i, field := range fields {
		fields[i] = romaji.NormalizeExact(field)
	}
	return strings.Join(fields, "")
}

func foldFullwidth(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\u3000':
			return ' '
		case r >= '\uff01' && r <= '\uff5e':
			return r - 0xfee0
		default:
			return r
		}
	}, s)
}

func isDigitRune(r rune) bool { return r >= '0' && r <= '9' }

func digitsToKanji(s string) string {
	var out strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); {
		if !isDigitRune(runes[i]) {
			out.WriteRune(runes[i])
			i++
			continue
		}
		start := i
		for i < len(runes) && isDigitRune(runes[i]) {
			i++
		}
		out.WriteString(numberToKanji(string(runes[start:i])))
	}
	return out.String()
}

var kanjiDigits = []string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
var kanjiUnits = []struct {
	value int
	kanji string
}{
	{10000000, "千万"}, {1000000, "百万"}, {100000, "十万"},
	{10000, "万"}, {1000, "千"}, {100, "百"}, {10, "十"},
}

func numberToKanji(digits string) string {
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return "零"
	}
	value := 0
	for _, r := range digits {
		value = value*10 + int(r-'0')
	}
	return valueToKanji(value)
}

func valueToKanji(value int) string {
	if value == 0 {
		return "零"
	}
	var out strings.Builder
	for _, unit := range kanjiUnits {
		if value < unit.value {
			continue
		}
		count := value / unit.value
		value %= unit.value
		if count > 1 || unit.value >= 10000 {
			out.WriteString(kanjiDigits[count])
		}
		out.WriteString(unit.kanji)
	}
	if value > 0 {
		out.WriteString(kanjiDigits[value])
	}
	return out.String()
}
