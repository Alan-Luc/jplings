package romaji

import "testing"

func TestCompareAcceptsSpellingConventions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		accepted string
	}{
		{"hepburn versus wapuro s", "susi", "sushi"},
		{"tsu versus tu", "tunami", "tsunami"},
		{"fu versus hu", "huzi", "fuji"},
		{"ji versus zi", "zi", "ji"},
		{"small ya digraph", "jya", "ja"},
		{"chi versus ti", "tizu", "chizu"},
		{"case and spacing", "  SuShi   o kudasai ", "sushi o kudasai"},
		{"macron and doubled vowel", "obāsan", "obaasan"},
		{"particle を as wo or o", "mizu wo kudasai", "mizu o kudasai"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := Compare(tc.input, []string{tc.accepted})
			if got != Exact {
				t.Fatalf("Compare(%q, %q) = %v, want Exact", tc.input, tc.accepted, got)
			}
		})
	}
}

func TestCompareDoesNotMangleWordsContainingWo(t *testing.T) {
	if got, _ := Compare("koai", []string{"kowai"}); got == Exact {
		t.Fatal("kowai must not be normalized to koai")
	}
}

func TestCompareNeverAwardsCreditForDifferentWord(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		accepted   string
		want       Verdict
		wantReason string
	}{
		{"vowel length", "obasan", "obaasan", Close, "vowel length"},
		{"gemination", "hasen", "hassen", Close, "gemination"},
		{"kana gemination", "はせん", "はっせん", Close, "gemination"},
		{"unrelated", "neko", "inu", Wrong, ""},
		{"kana vowel length is not guessed", "おばさん", "おばあさん", Wrong, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verdict, reason := Compare(tc.input, []string{tc.accepted})
			if verdict != tc.want {
				t.Fatalf("Compare(%q, %q) = %v, want %v", tc.input, tc.accepted, verdict, tc.want)
			}
			if reason != tc.wantReason {
				t.Fatalf("Compare(%q, %q) reason = %q, want %q", tc.input, tc.accepted, reason, tc.wantReason)
			}
		})
	}
}

func TestCompareMatchesAnyAcceptedAnswer(t *testing.T) {
	accepted := []string{"mizu o kudasai", "omizu o kudasai"}
	if got, _ := Compare("omizu wo kudasai", accepted); got != Exact {
		t.Fatalf("expected the second accepted answer to match, got %v", got)
	}
}

func TestNormalizeExactKeepsDifferentWordsApart(t *testing.T) {
	if NormalizeExact("obasan") == NormalizeExact("obaasan") {
		t.Fatal("exact normalization must not merge おばさん and おばあさん")
	}
	if NormalizeClose("obasan") != NormalizeClose("obaasan") {
		t.Fatal("close normalization should treat them as a near miss")
	}
}

func TestCompareReportsCombinedDistinction(t *testing.T) {
	if _, reason := Compare("kite", []string{"kitte"}); reason != "gemination" {
		t.Fatalf("reason = %q, want gemination", reason)
	}
}

func TestKatakanaLongVowelNeverEarnsCredit(t *testing.T) {
	tests := []struct{ input, accepted string }{
		{"ビル", "ビール"},
		{"コヒ", "コーヒー"},
		{"ジュス", "ジュース"},
	}
	for _, tc := range tests {
		verdict, reason := Compare(tc.input, []string{tc.accepted})
		if verdict == Exact {
			t.Fatalf("Compare(%q, %q) earned credit; a long vowel makes it a different word", tc.input, tc.accepted)
		}
		if verdict != Close {
			t.Fatalf("Compare(%q, %q) = %v, want Close", tc.input, tc.accepted, verdict)
		}
		if reason != "vowel length" {
			t.Fatalf("reason = %q, want vowel length", reason)
		}
	}
}

func TestExactStillAcceptsLongVowelSpellingConventions(t *testing.T) {
	if got, _ := Compare("biiru", []string{"bīru"}); got != Exact {
		t.Fatalf("Compare(biiru, bīru) = %v, want Exact", got)
	}
}

func TestSpacingIsNotSignificant(t *testing.T) {
	for _, input := range []string{"mizu o kudasai", "mizuokudasai", "mizu  o   kudasai"} {
		if got, _ := Compare(input, []string{"mizu o kudasai"}); got != Exact {
			t.Fatalf("Compare(%q) = %v, want Exact", input, got)
		}
	}
	if got, _ := Compare("nomi masu", []string{"nomimasu"}); got != Exact {
		t.Fatalf("per-morpheme romaji must match an unspaced answer, got %v", got)
	}
}
