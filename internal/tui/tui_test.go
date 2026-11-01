package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jplings/internal/content"
	"jplings/internal/profile"
	"jplings/internal/session"
)

func testUnits() []*content.Unit {
	return []*content.Unit{{
		ID:    "u",
		Title: "Unit",
		Lessons: []content.Lesson{{
			ID:    "l",
			Title: "Lesson",
			Steps: []content.Step{
				{
					ID: "s1", Kind: content.Transformation, Concepts: []string{"c1"},
					Prompt:      "Say the thing politely.",
					Target:      &content.Text{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"},
					Accept:      []content.Text{{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"}},
					Explanation: "を marks the object.",
				},
				{
					ID: "s2", Kind: content.Construction, Concepts: []string{"c1"},
					Prompt:      "Say it again differently.",
					Accept:      []content.Text{{Kanji: "お水をください", Kana: "おみずをください", Romaji: "omizu o kudasai"}},
					Explanation: "Politeness is optional here.",
				},
			},
		}},
	}}
}

func newModel(t *testing.T) (Model, *profile.Store) {
	t.Helper()
	store, err := profile.Open(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	runner := &session.Runner{Store: store}
	return New(runner, testUnits()), store
}

func TestViewShowsTheFirstStep(t *testing.T) {
	model, _ := newModel(t)
	view := model.View()

	if !strings.Contains(view, "Say the thing politely.") {
		t.Fatalf("view should show the prompt, got:\n%s", view)
	}
	if !strings.Contains(view, "水をください") {
		t.Fatalf("view should show the Japanese target, got:\n%s", view)
	}
}

func TestSubmitThenAdvance(t *testing.T) {
	model, store := newModel(t)

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

	after.advance()
	if after.pos.step != 1 {
		t.Fatalf("expected to advance to step 2, at %d", after.pos.step)
	}
}

func TestSessionCompletesAfterLastStep(t *testing.T) {
	model, _ := newModel(t)
	model.pos.step = 1
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

	if err := store.SaveCursor("u", "l", 1, time.Now()); err != nil {
		t.Fatalf("SaveCursor: %v", err)
	}

	model := New(&session.Runner{Store: store}, testUnits())
	if model.pos.step != 1 {
		t.Fatalf("expected to resume at step 1, got %d", model.pos.step)
	}
}
