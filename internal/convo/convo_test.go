package convo

import (
	"testing"

	"jplings/internal/content"
)

func scenario() *content.Scenario {
	return &content.Scenario{Turns: []content.Turn{
		{Speaker: "npc", Line: &content.Text{Kana: "いらっしゃいませ", Romaji: "irasshaimase"}},
		{Speaker: "learner", Accept: []content.Text{
			{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"},
			{Kanji: "お水をください", Kana: "おみずをください", Romaji: "omizu o kudasai"},
		}},
		{Speaker: "npc", Line: &content.Text{Kana: "はい、どうぞ", Romaji: "hai, douzo"}},
	}}
}

func TestMatchedTurnEarnsCredit(t *testing.T) {
	c := New(scenario())
	c.Advance()

	if got := c.Respond("水をください。").Verdict; got != Matched {
		t.Fatalf("verdict = %v, want Matched", got)
	}
	if got := c.Respond("お水をください").Verdict; got != Matched {
		t.Fatalf("politeness variant should match an accepted answer, got %v", got)
	}
}

func TestDroppedParticleIsToleratedAndHinted(t *testing.T) {
	c := New(scenario())
	c.Advance()

	got := c.Respond("みずください")
	if got.Verdict != Tolerated {
		t.Fatalf("verdict = %v, want Tolerated", got.Verdict)
	}
	if got.Hint == "" {
		t.Fatal("a tolerated turn must explain what was dropped")
	}
}

func TestUnrelatedTurnIsNoMatch(t *testing.T) {
	c := New(scenario())
	c.Advance()

	if got := c.Respond("ありがとうございます").Verdict; got != NoMatch {
		t.Fatalf("verdict = %v, want NoMatch", got)
	}
}

func TestRespondOnNpcTurnIsNoMatch(t *testing.T) {
	c := New(scenario())
	if got := c.Respond("水をください").Verdict; got != NoMatch {
		t.Fatalf("verdict = %v, want NoMatch on an NPC turn", got)
	}
}

func TestWalksScenarioToCompletion(t *testing.T) {
	c := New(scenario())
	for !c.Done() {
		c.Advance()
	}
	if !c.Done() {
		t.Fatal("scenario should be done after all turns")
	}
	if _, ok := c.Current(); ok {
		t.Fatal("no turn should remain")
	}
}
