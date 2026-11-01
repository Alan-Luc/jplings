package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"jplings/internal/content"
	"jplings/internal/profile"
	"jplings/internal/session"
	"jplings/internal/speech"
)

func listeningUnits() []*content.Unit {
	return []*content.Unit{{
		ID:    "u",
		Title: "Unit",
		Lessons: []content.Lesson{{
			ID:    "l",
			Title: "Lesson",
			Steps: []content.Step{{
				ID: "s1", Kind: content.Listening, Concepts: []string{"c1"},
				Prompt: "Listen and type the price you hear.",
				Target: &content.Text{Kanji: "六百円です", Kana: "ろっぴゃくえんです", Romaji: "roppyaku en desu"},
				Accept: []content.Text{
					{Kanji: "六百円", Kana: "ろっぴゃくえん", Romaji: "roppyaku en"},
				},
				Explanation: "六百 is ろっぴゃく.",
			}},
		}},
	}}
}

func readingUnits() []*content.Unit {
	return []*content.Unit{{
		ID:    "u",
		Title: "Unit",
		Lessons: []content.Lesson{{
			ID:    "l",
			Title: "Lesson",
			Steps: []content.Step{{
				ID: "s1", Kind: content.Transformation, Concepts: []string{"c1"},
				Prompt:      "Say the thing politely.",
				Target:      &content.Text{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"},
				Accept:      []content.Text{{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"}},
				Explanation: "を marks the object.",
			}},
		}},
	}}
}

func newModelWith(t *testing.T, units []*content.Unit) (Model, *profile.Store) {
	t.Helper()
	store, err := profile.Open(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	runner := &session.Runner{Store: store, Client: &speech.Mock{}}
	return New(runner, units), store
}

// A listening exercise is audio-first: the answer must not be on screen before the
// attempt, or the exercise measures reading instead of listening.
func TestListeningTargetIsHiddenUntilAnswered(t *testing.T) {
	model, _ := newModelWith(t, listeningUnits())

	before := model.View()
	if strings.Contains(before, "六百円です") {
		t.Fatalf("the listening target must not be visible before answering:\n%s", before)
	}
	if !strings.Contains(before, "六百円です\n") && !strings.Contains(before, "listen") {
		t.Fatalf("expected a listening prompt:\n%s", before)
	}

	updated, _ := model.submit("roppyaku en")
	after := updated.(Model)

	revealed := after.View()
	if !strings.Contains(revealed, "六百円 is ろっぴゃく") && !strings.Contains(revealed, "六百") {
		t.Fatalf("the answer should be revealed in feedback:\n%s", revealed)
	}
}

// A reading exercise still shows its target immediately.
func TestReadingTargetIsAlwaysVisible(t *testing.T) {
	model, _ := newModelWith(t, readingUnits())
	if !strings.Contains(model.View(), "水をください") {
		t.Fatalf("a transformation target should be visible:\n%s", model.View())
	}
}

func TestAudioTextOnlySpeaksListeningTargets(t *testing.T) {
	listening := listeningUnits()[0].Lessons[0].Steps[0]
	if got := audioText(&listening); got != "六百円です" {
		t.Fatalf("audioText(listening) = %q, want the kanji target", got)
	}

	reading := readingUnits()[0].Lessons[0].Steps[0]
	if got := audioText(&reading); got != "" {
		t.Fatalf("audioText(transformation) = %q, want empty so the answer is not spoken", got)
	}

	speaking := content.Step{Kind: content.Speaking, Target: &content.Text{Kanji: "水", Kana: "みず"}}
	if got := audioText(&speaking); got != "" {
		t.Fatalf("audioText(speaking) = %q, want empty so the answer is not given away", got)
	}
}

// Each step speaks once; a re-render must not re-synthesize the same line.
func TestPlayCurrentIsIdempotentPerStep(t *testing.T) {
	model, _ := newModelWith(t, listeningUnits())

	if cmd := model.playCurrent(); cmd == nil {
		t.Fatal("expected a playback command for a listening step")
	}
	if cmd := model.playCurrent(); cmd != nil {
		t.Fatal("the same step must not be played twice")
	}

	model.spokenStep = ""
	if cmd := model.playCurrent(); cmd == nil {
		t.Fatal("clearing spokenStep should allow a replay")
	}
}

func TestDegradedAudioShowsTheWrittenPrompt(t *testing.T) {
	model, _ := newModelWith(t, listeningUnits())
	model.degraded = true

	view := model.View()
	if !strings.Contains(view, "audio unavailable") {
		t.Fatalf("degraded audio must be labelled:\n%s", view)
	}
	if !strings.Contains(view, "六百円です") {
		t.Fatalf("degraded audio must reveal the written prompt:\n%s", view)
	}
}

func TestViewShowsTheFirstStep(t *testing.T) {
	model, _ := newModelWith(t, readingUnits())
	view := model.View()

	if !strings.Contains(view, "Say the thing politely.") {
		t.Fatalf("view should show the prompt, got:\n%s", view)
	}
	if !strings.Contains(view, "水をください") {
		t.Fatalf("view should show the Japanese target, got:\n%s", view)
	}
}

func TestSubmitThenAdvance(t *testing.T) {
	model, store := newModelWith(t, readingUnits())

	updated, _ := model.submit("mizu o kudasai")
	after := updated.(Model)
	if after.screen != screenFeedback {
		t.Fatalf("expected the feedback screen, got %v", after.screen)
	}
	if !after.correct {
		t.Fatalf("expected a correct verdict, got %q", after.feedback)
	}

	evidence, err := store.Evidence("c1")
	if err != nil || len(evidence) != 1 {
		t.Fatalf("expected evidence through the TUI path, got %+v err=%v", evidence, err)
	}
}

func TestSessionCompletesAfterLastStep(t *testing.T) {
	model, _ := newModelWith(t, readingUnits())
	model.advance()

	if model.screen != screenDone {
		t.Fatalf("expected the done screen, got %v", model.screen)
	}
	if !strings.Contains(model.View(), "Session complete") {
		t.Fatalf("done view missing completion text:\n%s", model.View())
	}
}

func TestCursorResumesPosition(t *testing.T) {
	store, err := profile.Open(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	units := readingUnits()
	units[0].Lessons[0].Steps = append(units[0].Lessons[0].Steps,
		content.Step{
			ID: "s2", Kind: content.Construction, Concepts: []string{"c1"}, Prompt: "Second.",
			Accept:      []content.Text{{Kana: "みず", Romaji: "mizu"}},
			Explanation: "because",
		})

	if err := store.SaveCursor("u", "l", 1, time.Now()); err != nil {
		t.Fatalf("SaveCursor: %v", err)
	}

	model := New(&session.Runner{Store: store}, units)
	if model.pos.step != 1 {
		t.Fatalf("expected to resume at step 1, got %d", model.pos.step)
	}
}

// Regression: replay used to be bound to the letter "a", which silently swallowed
// every "a" typed into a romaji answer.
func TestTypingALetterInsertsItInsteadOfTriggeringAHotkey(t *testing.T) {
	for _, letter := range []rune{'a', 'r', 'w', 's'} {
		model, _ := newModelWith(t, readingUnits())

		updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{letter}})
		got := updated.(Model).input.Value()

		if got != string(letter) {
			t.Fatalf("typing %q produced %q in the input", string(letter), got)
		}
	}
}

func TestTypingAFullAnswerWorks(t *testing.T) {
	model, _ := newModelWith(t, readingUnits())

	for _, letter := range "watashi wa gakusei desu" {
		if letter == ' ' {
			updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
			model = updated.(Model)
			continue
		}
		updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{letter}})
		model = updated.(Model)
	}

	if got := model.input.Value(); got != "watashi wa gakusei desu" {
		t.Fatalf("typed answer = %q", got)
	}
}

// Regression: the spacebar arrives as a space rune, so matching the literal string
// "space" never matched and recording could not be stopped.
func TestSpacebarIsRecognised(t *testing.T) {
	if !isSpace(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}) {
		t.Fatal("a space key press must be recognised")
	}
	if isSpace(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}) {
		t.Fatal("the letter a is not a space")
	}
}

func speakingUnits() []*content.Unit {
	return []*content.Unit{{ID: "u", Title: "Unit", Lessons: []content.Lesson{{ID: "l", Title: "Lesson",
		Steps: []content.Step{{ID: "s1", Kind: content.Speaking, Concepts: []string{"c1"},
			Prompt: "Say it.", Accept: []content.Text{{Kana: "みず", Romaji: "mizu"}},
			Explanation: "because"}}}}}}
}

func TestSpacebarStopsRecording(t *testing.T) {
	model, _ := newModelWith(t, speakingUnits())

	stopped := false
	model.recording = true
	model.cancelRecord = func() { stopped = true }

	updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if !stopped {
		t.Fatal("pressing space during recording must cancel it")
	}
	if updated.(Model).cancelRecord != nil {
		t.Fatal("the cancel handle should be cleared after stopping")
	}
}

func conversationUnits() []*content.Unit {
	return []*content.Unit{{ID: "u", Title: "Unit", Lessons: []content.Lesson{{ID: "l", Title: "L",
		Steps: []content.Step{{
			ID: "c1", Kind: content.Conversation, Concepts: []string{"kudasai"},
			Prompt: "Order water at a restaurant.",
			Scenario: &content.Scenario{Turns: []content.Turn{
				{Speaker: "npc", Line: &content.Text{Kanji: "いらっしゃいませ", Kana: "いらっしゃいませ", Romaji: "irasshaimase"}},
				{Speaker: "learner", Accept: []content.Text{
					{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"},
				}},
				{Speaker: "npc", Line: &content.Text{Kanji: "はい、どうぞ", Kana: "はい、どうぞ", Romaji: "hai douzo"}},
			}},
			Explanation: "をください is the request pattern.",
		}},
	}}}}
}

// Regression from review: a conversation step used to be graded by the answer
// graders against an empty accept list, so content shipping a conversation could
// never be completed.
func TestConversationStepIsCompletable(t *testing.T) {
	model, store := newModelWith(t, conversationUnits())

	if !strings.Contains(model.View(), "いらっしゃいませ") {
		t.Fatalf("the NPC's line should be shown:\n%s", model.View())
	}

	updated, _ := model.submit("水をください。")
	after := updated.(Model)
	if !after.correct {
		t.Fatalf("an accepted conversation answer should earn credit, got %q", after.feedback)
	}
	if after.screen != screenFeedback {
		t.Fatalf("expected the feedback screen, got %v", after.screen)
	}

	evidence, err := store.Evidence("kudasai")
	if err != nil || len(evidence) != 1 {
		t.Fatalf("a matched turn should mint evidence, got %+v err=%v", evidence, err)
	}
	if !strings.Contains(after.feedback, "をください") {
		t.Fatalf("feedback should explain the pattern, got %q", after.feedback)
	}

	// The NPC's reply after the learner's turn is collected for the next render.
	if len(after.npcLines) != 2 {
		t.Fatalf("expected both NPC lines, got %d", len(after.npcLines))
	}
}

func TestToleratedConversationTurnMintsNoEvidence(t *testing.T) {
	model, store := newModelWith(t, conversationUnits())

	updated, _ := model.submit("みずください")
	after := updated.(Model)
	if after.correct {
		t.Fatal("a tolerated turn must not earn credit")
	}
	if after.feedback == "" {
		t.Fatal("a tolerated turn must explain what was dropped")
	}
	if evidence, _ := store.Evidence("kudasai"); len(evidence) != 0 {
		t.Fatalf("a tolerated turn must mint no evidence, got %+v", evidence)
	}
}
