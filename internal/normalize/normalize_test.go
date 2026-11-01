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

func TestSurfaceKeepsRomajiParticleUnfolded(t *testing.T) {
	if got := Surface("mizu o tabemashita"); got != "mizuotabemashita" {
		t.Fatalf("Surface(%q) = %q, want the romaji particle left as o", "mizu o tabemashita", got)
	}
	if got := Surface("mizu wo tabemashita"); got != "mizuotabemashita" {
		t.Fatalf("Surface(%q) = %q, want wo folded to o without kana folding", "mizu wo tabemashita", got)
	}
}
