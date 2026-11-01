package corpus

import (
	"strings"
	"testing"
)

const jmdictPriorityFixture = `<?xml version="1.0" encoding="UTF-8"?>
<JMdict>
<entry>
  <ent_seq>1</ent_seq>
  <k_ele><keb>水</keb><ke_pri>news1</ke_pri><ke_pri>nf05</ke_pri></k_ele>
  <r_ele><reb>みず</reb></r_ele>
  <sense><gloss>water</gloss></sense>
</entry>
<entry>
  <ent_seq>2</ent_seq>
  <k_ele><keb>猫</keb></k_ele>
  <r_ele><reb>ねこ</reb><re_pri>nf12</re_pri></r_ele>
  <sense><gloss>cat</gloss></sense>
</entry>
<entry>
  <ent_seq>3</ent_seq>
  <k_ele><keb>犬</keb><ke_pri>gai1</ke_pri></k_ele>
  <r_ele><reb>いぬ</reb></r_ele>
  <sense><gloss>dog</gloss></sense>
</entry>
<entry>
  <ent_seq>4</ent_seq>
  <k_ele><keb>鳥</keb></k_ele>
  <r_ele><reb>とり</reb></r_ele>
  <sense><gloss>bird</gloss></sense>
</entry>
</JMdict>`

func TestLoadJMdictExtractsFrequencyBand(t *testing.T) {
	lookup, err := LoadJMdict(strings.NewReader(jmdictPriorityFixture),
		map[string]bool{"水": true, "猫": true, "犬": true, "鳥": true})
	if err != nil {
		t.Fatalf("LoadJMdict: %v", err)
	}

	for form, want := range map[string]int{"水": 1, "猫": 12, "犬": 5, "鳥": 0} {
		if got := lookup[form].Frequency; got != want {
			t.Errorf("%s frequency = %d, want %d", form, got, want)
		}
	}
}

func TestResolveWordsRejectsTruncatedWordList(t *testing.T) {
	_, _, err := ResolveWords(strings.NewReader(jmdictPriorityFixture),
		map[string]bool{"水": true, "存在しない": true})
	if err == nil {
		t.Fatal("a shorter-than-requested word list must fail loudly")
	}
	for _, want := range []string{"resolved 1 of 2", "存在しない"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q must mention %q", err, want)
		}
	}
}

func TestResolveWordsAcceptsAWholeWordList(t *testing.T) {
	_, words, err := ResolveWords(strings.NewReader(jmdictPriorityFixture),
		map[string]bool{"水": true, "猫": true})
	if err != nil {
		t.Fatalf("ResolveWords: %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("words = %d, want 2", len(words))
	}
}

func TestRankCandidatesRewardsKnownWordCoverage(t *testing.T) {
	known := Lookup{
		"水":  {Kana: []string{"みず"}, Frequency: 2},
		"飲む": {Kana: []string{"のむ"}, Frequency: 3},
	}
	candidates := RankCandidates([]Sentence{
		{Japanese: "猫が好きです。", ID: "2"},
		{Japanese: "水を飲む。", ID: "1"},
	}, known, nil, 0)

	if len(candidates) != 2 {
		t.Fatalf("candidates = %d, want 2", len(candidates))
	}
	if candidates[0].ID != "1" {
		t.Fatalf("candidate order = %+v, want full-coverage sentence first", candidates)
	}
	if candidates[0].Coverage != 0.75 {
		t.Fatalf("coverage = %v, want 0.75", candidates[0].Coverage)
	}
	if candidates[1].Coverage != 0 {
		t.Fatalf("unknown sentence coverage = %v, want 0", candidates[1].Coverage)
	}
}

func TestRankCandidatesBreaksCoverageTiesByFrequencyBand(t *testing.T) {
	known := Lookup{
		"水": {Kana: []string{"みず"}, Frequency: 2},
		"猫": {Kana: []string{"ねこ"}, Frequency: 5},
		"犬": {Kana: []string{"いぬ"}, Frequency: 0},
	}
	candidates := RankCandidates([]Sentence{
		{Japanese: "犬だ", ID: "3"},
		{Japanese: "猫だ", ID: "2"},
		{Japanese: "水だ", ID: "1"},
	}, known, nil, 0)

	if len(candidates) != 3 {
		t.Fatalf("candidates = %d, want 3", len(candidates))
	}
	for i, want := range []string{"1", "2", "3"} {
		if candidates[i].ID != want {
			t.Fatalf("order = %v, want %s at position %d", []string{
				candidates[0].ID, candidates[1].ID, candidates[2].ID,
			}, want, i)
		}
	}
}

func TestRankCandidatesDedupesExistingContent(t *testing.T) {
	candidates := RankCandidates([]Sentence{
		{Japanese: "水をください。", ID: "1"},
		{Japanese: "猫が好きです。", ID: "2"},
	}, nil, []string{"水をください"}, 0)

	if len(candidates) != 1 || candidates[0].ID != "2" {
		t.Fatalf("candidates = %+v, want only the sentence absent from content", candidates)
	}
}

func TestRankCandidatesDedupesRepeatedSentences(t *testing.T) {
	candidates := RankCandidates([]Sentence{
		{Japanese: "水をください。", ID: "1"},
		{Japanese: "水をください", ID: "2"},
	}, nil, nil, 0)

	if len(candidates) != 1 {
		t.Fatalf("candidates = %+v, want one normalized sentence", candidates)
	}
}

func TestRankCandidatesRespectsLimit(t *testing.T) {
	candidates := RankCandidates([]Sentence{
		{Japanese: "水をください。", ID: "1"},
		{Japanese: "猫が好きです。", ID: "2"},
	}, nil, nil, 1)

	if len(candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(candidates))
	}
}

func TestWriteCandidatesCarriesScoresAndLicence(t *testing.T) {
	var out strings.Builder
	err := WriteCandidates(&out, []Candidate{{
		Sentence: Sentence{Japanese: "水をください。", English: "Water, please.", ID: "1"},
		Coverage: 0.75,
		Band:     2,
		Matched:  []string{"水"},
	}})
	if err != nil {
		t.Fatalf("WriteCandidates: %v", err)
	}

	rendered := out.String()
	for _, want := range []string{"CC BY-SA 4.0", "CC BY 2.0 FR", "coverage: 0.75", "band: 2", "matched_words"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("shortlist must contain %q, got:\n%s", want, rendered)
		}
	}
}
