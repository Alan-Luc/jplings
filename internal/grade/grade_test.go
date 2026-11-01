package grade

import (
	"strings"
	"testing"

	"jplings/internal/content"
)

func constructionStep() *content.Step {
	return &content.Step{
		ID:   "s1",
		Kind: content.Construction,
		Accept: []content.Text{
			{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"},
		},
		Explanation: "を marks the object of ください.",
	}
}

func TestWrittenAcceptsRomajiKanaAndKanjiSurfaces(t *testing.T) {
	step := constructionStep()
	for _, answer := range []string{"mizu o kudasai", "みずをください", "水をください", "mizu wo kudasai"} {
		out := Written(step, answer)
		if !out.Correct {
			t.Fatalf("Written(%q) should be correct, got %+v", answer, out)
		}
	}
}

func TestWrittenCloseNeverEarnsCredit(t *testing.T) {
	step := &content.Step{
		Kind:        content.Speaking,
		Accept:      []content.Text{{Kana: "はっせん", Romaji: "hassen"}},
		Explanation: "はっせん is 8000.",
	}
	out := Written(step, "hasen")
	if out.Correct {
		t.Fatal("a vowel/gemination near miss must not earn credit")
	}
	if !out.Close {
		t.Fatal("expected the near miss to be recognized as close")
	}
	if !strings.Contains(out.Explanation, "gemination") {
		t.Fatalf("explanation should name gemination, got %q", out.Explanation)
	}
}

func TestWrittenOffTargetIsValidJapaneseButNoCredit(t *testing.T) {
	step := &content.Step{
		Kind:        content.Transformation,
		Accept:      []content.Text{{Kana: "のみたいです", Romaji: "nomitai desu"}},
		OffTarget:   []content.OffTarget{{Text: content.Text{Kana: "のめます", Romaji: "nomemasu"}, Meaning: "I can drink"}},
		Explanation: "たい attaches to the verb stem.",
	}
	out := Written(step, "nomemasu")
	if out.Correct {
		t.Fatal("an off-target form must not earn credit")
	}
	if !out.OffTarget {
		t.Fatal("expected the answer to be flagged off-target")
	}
	if !strings.Contains(out.Explanation, "I can drink") {
		t.Fatalf("explanation should name the actual meaning, got %q", out.Explanation)
	}
}

func TestWrittenWrongAlwaysExplains(t *testing.T) {
	out := Written(constructionStep(), "neko")
	if out.Correct {
		t.Fatal("unrelated answer must not be correct")
	}
	if out.Explanation == "" {
		t.Fatal("feedback must never be a bare incorrect")
	}
}

func TestMeaning(t *testing.T) {
	step := &content.Step{
		Kind:           content.Interpretation,
		AcceptMeanings: []string{"The student is Tanaka.", "Tanaka is the student."},
		Explanation:    "は puts 学生 in the topic slot.",
	}
	for _, answer := range []string{"the student is tanaka", "Tanaka is the student."} {
		if out := Meaning(step, answer); !out.Correct {
			t.Fatalf("Meaning(%q) should be correct, got %+v", answer, out)
		}
	}
	out := Meaning(step, "Tanaka is a teacher")
	if out.Correct || out.Explanation == "" {
		t.Fatalf("wrong meaning must be incorrect and explained, got %+v", out)
	}
}

func spokenStep() *content.Step {
	return &content.Step{
		Kind: content.Speaking,
		Accept: []content.Text{
			{Kanji: "六百円", Kana: "ろっぴゃくえん", Romaji: "roppyaku en"},
		},
		Explanation: "六百円 is 600 yen.",
	}
}

// Digits, kanji numerals and kana spellings all name the same amount.
func TestSpokenAcceptsEveryNumeralSpelling(t *testing.T) {
	step := spokenStep()
	for _, transcript := range []string{"600円", "六百円", "６００円", "600 円"} {
		if out := Spoken(step, transcript); !out.Correct {
			t.Fatalf("Spoken(%q) should be correct, got %+v", transcript, out)
		}
	}
}

// omitting the counter is a specific error, not a correct answer.
func TestSpokenRequiresTheCounter(t *testing.T) {
	out := Spoken(spokenStep(), "600")
	if out.Correct {
		t.Fatal("a missing counter must not earn credit")
	}
	if !strings.Contains(out.Explanation, "counter is missing") {
		t.Fatalf("explanation should name the missing counter, got %q", out.Explanation)
	}
}

func TestSpokenDifferentWordIsWrongNotPronunciation(t *testing.T) {
	step := spokenStep()
	out := Spoken(step, "八百円")
	if out.Correct {
		t.Fatal("a different amount is a different word and must not pass")
	}
	if strings.Contains(strings.ToLower(out.Explanation), "pronunciation") {
		t.Fatalf("word identity errors must not be framed as pronunciation, got %q", out.Explanation)
	}
}

func TestNumberCanonicalization(t *testing.T) {
	tests := map[string]string{
		"600円":  "六百円",
		"8000円": "八千円",
		"3500円": "三千五百円",
		"１０円":   "十円",
		"600":   "六百",
		"3500":  "三千五百",
	}
	for input, want := range tests {
		if got := normalizeJapan(input); got != want {
			t.Fatalf("normalizeJapan(%q) = %q, want %q", input, got, want)
		}
	}
}
