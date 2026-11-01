package corpus

import "testing"

func TestBuildReadingSeparatesSpellingFromPronunciation(t *testing.T) {
	morphemes := []Morpheme{
		{String: "私", Read: "ワタシ", Pron: "ワタシ", Pos: "名詞"},
		{String: "は", Read: "ハ", Pron: "ワ", Pos: "助詞"},
		{String: "水", Read: "ミズ", Pron: "ミズ", Pos: "名詞"},
		{String: "を", Read: "ヲ", Pron: "ヲ", Pos: "助詞"},
		{String: "。", Read: "、", Pron: "、", Pos: "記号"},
	}
	got := BuildReading(morphemes)

	if got.Kana != "わたしはみずを。" {
		t.Fatalf("kana = %q, want the particle spelled は", got.Kana)
	}
	if got.Romaji != "watashi wa mizu o" {
		t.Fatalf("romaji = %q, want the particle pronounced wa", got.Romaji)
	}
}

func TestBuildReadingKeepsOriginalPunctuation(t *testing.T) {
	got := BuildReading([]Morpheme{
		{String: "寒い", Read: "サムイ", Pron: "サムイ", Pos: "形容詞"},
		{String: "。", Read: "、", Pron: "、", Pos: "記号"},
	})
	if got.Kana != "さむい。" {
		t.Fatalf("kana = %q, the analyser normalises 。 to 、 and that must not leak", got.Kana)
	}
}

func TestEnrichFailsLoudlyWhenServiceIsDown(t *testing.T) {
	reader := &Reader{URL: "http://127.0.0.1:1/reading"}
	if _, err := Enrich(t.Context(), reader, []Sentence{{Japanese: "寒い。", ID: "1"}}); err == nil {
		t.Fatal("a missing reading service must error rather than export blank readings")
	}
}
