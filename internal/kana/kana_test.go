package kana

import "testing"

func TestToHiragana(t *testing.T) {
	tests := map[string]string{
		"ワタシ":     "わたし",
		"ミズヲノミマス": "みずをのみます",
		"ヴァイオリン":  "ゔぁいおりん",
		"abc日本":   "abc日本",
	}
	for input, want := range tests {
		if got := ToHiragana(input); got != want {
			t.Fatalf("ToHiragana(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestToRomaji(t *testing.T) {
	tests := map[string]string{
		"ワタシワ":    "watashiwa",
		"ミズヲノミマス": "mizuonomimasu",
		"キャット":    "kyatto",
		"シャシン":    "shashin",
		"チャ":      "cha",
		"ジュース":    "juusu",
		"コーヒー":    "koohii",
		"ガッコウ":    "gakkou",
		"コンピューター": "konpyuutaa",
		"ファミリー":   "famirii",
		"ティー":     "tii",
	}
	for input, want := range tests {
		if got := ToRomaji(input); got != want {
			t.Fatalf("ToRomaji(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestToRomajiIgnoresUnknownRunes(t *testing.T) {
	if got := ToRomaji("ミズ。"); got != "mizu" {
		t.Fatalf("punctuation should be skipped, got %q", got)
	}
}
