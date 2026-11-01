package normalize

import "testing"

func TestSurfaceFoldsPunctuationWhitespaceAndNumbers(t *testing.T) {
	tests := map[string]string{
		"  水を ください。 ":     "水をください",
		"８００円":            "八百円",
		"ＡＢＣ！":            "abc",
		"ビール":             "ビール",
		"mizu wo kudasai": "mizuokudasai",
		"susi":            "sushi",
		"Tōkyō":           "toukyou",
	}
	for input, want := range tests {
		if got := Surface(input); got != want {
			t.Fatalf("Surface(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestKanaFoldsKatakanaAndRomaji(t *testing.T) {
	tests := map[string]string{
		"ビール":             "びいる",
		"コーヒー":            "こおひい",
		"mizu o kudasai":  "みずおください",
		"mizu wo kudasai": "みずおください",
		"mizuwo kudasai":  "みずをください",
		"chu":             "ちゅ",
		"hu":              "ふ",
		"toukyou":         "とうきょう",
		"tōkyō":           "とうきょう",
		"８００円":            "八百円",
	}
	for input, want := range tests {
		if got := Kana(input); got != want {
			t.Fatalf("Kana(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSurfaceKeepsRomajiParticleBeforeKanaFolding(t *testing.T) {
	if got := Surface("mizu o tabemashita"); got != "mizuotabemashita" {
		t.Fatalf("Surface(%q) = %q, want the romaji particle left as o", "mizu o tabemashita", got)
	}
	if got := Kana("mizu o tabemashita"); got != "みずおたべました" {
		t.Fatalf("Kana(%q) = %q, want the particle folded to お", "mizu o tabemashita", got)
	}
}
