package corpus

import (
	"strings"
	"testing"
)

const jmdictFixture = `<?xml version="1.0" encoding="UTF-8"?>
<JMdict>
<entry>
  <ent_seq>1000001</ent_seq>
  <k_ele><keb>水</keb></k_ele>
  <r_ele><reb>みず</reb></r_ele>
  <sense><pos>&n;</pos><gloss>water</gloss><gloss>&amp;H2O</gloss></sense>
  <sense><pos>&n;</pos><gloss>cold water</gloss></sense>
</entry>
<entry>
  <ent_seq>1000002</ent_seq>
  <k_ele><keb>飲む</keb></k_ele>
  <r_ele><reb>のむ</reb></r_ele>
  <sense><pos>&v5m;</pos><gloss>to drink</gloss></sense>
</entry>
<entry>
  <ent_seq>1000003</ent_seq>
  <k_ele><keb>猫</keb></k_ele>
  <r_ele><reb>ねこ</reb></r_ele>
  <sense><pos>&n;</pos><gloss>cat</gloss></sense>
</entry>
</JMdict>`

func TestLoadJMdictKeepsOnlyRequestedWords(t *testing.T) {
	lookup, err := LoadJMdict(strings.NewReader(jmdictFixture), map[string]bool{"水": true, "のむ": true})
	if err != nil {
		t.Fatalf("LoadJMdict: %v", err)
	}

	if _, ok := lookup["水"]; !ok {
		t.Fatal("水 should be present by kanji form")
	}
	if _, ok := lookup["のむ"]; !ok {
		t.Fatal("のむ should be present by kana form")
	}
	if _, ok := lookup["猫"]; ok {
		t.Fatal("猫 was not requested and must not be loaded")
	}
}

func TestLoadJMdictHandlesDTDEntities(t *testing.T) {
	_, err := LoadJMdict(strings.NewReader(jmdictFixture), map[string]bool{"水": true})
	if err != nil {
		t.Fatalf("named entities must not stop the parser: %v", err)
	}
}

func TestLoadJMdictDedupesPartOfSpeech(t *testing.T) {
	lookup, err := LoadJMdict(strings.NewReader(jmdictFixture), map[string]bool{"水": true})
	if err != nil {
		t.Fatalf("LoadJMdict: %v", err)
	}
	word := lookup["水"]
	if len(word.Pos) != 1 || word.Pos[0] != "n" {
		t.Fatalf("pos = %v, want a single deduped tag", word.Pos)
	}
	if len(word.Glosses) != 3 {
		t.Fatalf("glosses = %v, want all three senses", word.Glosses)
	}
}

func TestReadSentencesAndLinks(t *testing.T) {
	sentences, err := ReadSentences(strings.NewReader("1\tjpn\t水をください。\n2\tjpn\t猫が好きです。\n"))
	if err != nil {
		t.Fatalf("ReadSentences: %v", err)
	}
	if sentences["1"] != "水をください。" {
		t.Fatalf("sentences = %v", sentences)
	}

	links, err := ReadLinks(strings.NewReader("1\t10\n2\t20\n"))
	if err != nil {
		t.Fatalf("ReadLinks: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("links = %v", links)
	}
}

func TestPairsJoinsOnlyCompleteMatches(t *testing.T) {
	pairs := Pairs(
		map[string]string{"1": "水をください。", "2": "猫が好きです。"},
		map[string]string{"10": "Water, please."},
		[][2]string{{"1", "10"}, {"2", "20"}, {"3", "30"}},
	)
	if len(pairs) != 1 {
		t.Fatalf("pairs = %+v, want only the complete pair", pairs)
	}
	if pairs[0].Japanese != "水をください。" || pairs[0].English != "Water, please." || pairs[0].ID != "1" {
		t.Fatalf("pair = %+v", pairs[0])
	}
}

func TestPairsDedupesByJapaneseSentence(t *testing.T) {
	pairs := Pairs(
		map[string]string{"1": "寒い。"},
		map[string]string{"10": "It is cold.", "11": "It's cold."},
		[][2]string{{"1", "10"}, {"1", "11"}},
	)
	if len(pairs) != 1 {
		t.Fatalf("one Japanese sentence should yield one pair, got %+v", pairs)
	}
}

func TestSelectShortPrefersShortSentences(t *testing.T) {
	pairs := []Sentence{
		{Japanese: "これはとても長い日本語の文です。", ID: "3"},
		{Japanese: "寒い。", ID: "1"},
		{Japanese: "本当？", ID: "2"},
	}
	got := SelectShort(pairs, 2)
	if len(got) != 2 {
		t.Fatalf("got %d pairs, want 2", len(got))
	}
	for _, pair := range got {
		if pair.ID == "3" {
			t.Fatalf("the longest sentence should be dropped: %+v", got)
		}
	}
	if got[0].ID != "1" || got[1].ID != "2" {
		t.Fatalf("output should be ordered by id: %+v", got)
	}
}

func TestValidateRejectsEmpty(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Fatal("an empty import must fail loudly rather than emit nothing")
	}
}

func TestWriteVocabCarriesLicence(t *testing.T) {
	var out strings.Builder
	err := WriteVocab(&out, []*Word{{Kanji: []string{"水"}, Kana: []string{"みず"}, Glosses: []string{"water"}, Seq: "1"}})
	if err != nil {
		t.Fatalf("WriteVocab: %v", err)
	}

	rendered := out.String()
	for _, want := range []string{"CC BY-SA 4.0", "EDRDG", "https://"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("export must carry attribution %q, got:\n%s", want, rendered)
		}
	}
}

func TestFilterRejectsBlockedTerms(t *testing.T) {
	pairs := []Sentence{
		{Japanese: "死ね！", English: "Fuck you!", ID: "1"},
		{Japanese: "これは水です。", English: "This is water.", ID: "2"},
	}
	kept := Filter(pairs, FilterOptions{MinRunes: 4})
	if len(kept) != 1 || kept[0].ID != "2" {
		t.Fatalf("blocked term survived the filter: %+v", kept)
	}
}

func TestFilterRejectsVeryShortSentences(t *testing.T) {
	pairs := []Sentence{{Japanese: "寒い。", ID: "1"}, {Japanese: "今日は寒いです。", ID: "2"}}
	kept := Filter(pairs, FilterOptions{MinRunes: 6})
	if len(kept) != 1 || kept[0].ID != "2" {
		t.Fatalf("short sentence survived: %+v", kept)
	}
}

func TestFilterPrefersCuratedVocabulary(t *testing.T) {
	pairs := []Sentence{
		{Japanese: "彼は走る。", ID: "1"},
		{Japanese: "水をください。", ID: "2"},
	}
	kept := Filter(pairs, FilterOptions{MinRunes: 4, Require: map[string]bool{"水": true}})
	if len(kept) != 1 || kept[0].ID != "2" {
		t.Fatalf("curated-vocabulary filter kept the wrong sentence: %+v", kept)
	}
}

func TestFilterKeepsEverythingWithoutOptions(t *testing.T) {
	pairs := []Sentence{{Japanese: "あ。", ID: "1"}}
	if len(Filter(pairs, FilterOptions{})) != 1 {
		t.Fatal("the zero options should not reject anything")
	}
}
