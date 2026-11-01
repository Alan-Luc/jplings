package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"jplings/internal/content"
	"jplings/internal/profile"
	"jplings/internal/schedule"
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

func meaningUnits() []*content.Unit {
	return []*content.Unit{{
		ID:    "u",
		Title: "Unit",
		Lessons: []content.Lesson{{
			ID:    "l",
			Title: "Lesson",
			Steps: []content.Step{{
				ID: "s1", Kind: content.Listening, Concepts: []string{"c1"},
				Prompt: "What is being said about the place?",
				Target: &content.Text{
					Kanji: "駅の近くにコンビニがあります", Kana: "えきのちかくにコンビニがあります",
					Romaji: "eki no chikaku ni konbini ga arimasu",
				},
				AcceptMeanings: []string{"There is a convenience store near the station."},
				Explanation:    "に marks where the thing exists.",
			}},
		}},
	}}
}

func TestAudioTextSpeaksMeaningGradedTargets(t *testing.T) {
	hearing := meaningUnits()[0].Lessons[0].Steps[0]
	if got := audioText(&hearing); got != "駅の近くにコンビニがあります" {
		t.Fatalf("audioText(graded by meaning) = %q, want the kanji target", got)
	}

	reading := readingUnits()[0].Lessons[0].Steps[0]
	if got := audioText(&reading); got != "" {
		t.Fatalf("audioText(transformation) = %q, want empty so the answer is not spoken", got)
	}
}

func TestAudioTextSpeaksInterpretationTargets(t *testing.T) {
	step := content.Step{
		Kind:           content.Interpretation,
		Target:         &content.Text{Kanji: "学生は田中さんです", Kana: "がくせいはたなかさんです", Romaji: "gakusei wa tanaka san desu"},
		AcceptMeanings: []string{"The student is Tanaka."},
	}
	if got := audioText(&step); got != "学生は田中さんです" {
		t.Fatalf("audioText(interpretation) = %q, want the kanji target as the reference", got)
	}
}

func TestMissPlaysTheAcceptedAnswer(t *testing.T) {
	model, _ := newModelWith(t, readingUnits())

	updated, cmd := model.submit("mizu o kudasai desu")
	after := updated.(Model)
	if after.correct {
		t.Fatalf("expected a miss, got %q", after.feedback)
	}
	if cmd == nil {
		t.Fatal("a miss on a production step must play the accepted answer")
	}

	correct, _ := newModelWith(t, readingUnits())
	if _, cmd := correct.submit("mizu o kudasai"); cmd != nil {
		t.Fatal("a correct answer must not play the answer audio")
	}
}

func TestListeningMissRevealsTheTarget(t *testing.T) {
	dictation := listeningUnits()
	model, _ := newModelWith(t, dictation)

	updated, cmd := model.submit("roppyaku en desu")
	after := updated.(Model)
	if after.correct {
		t.Fatalf("expected a miss, got %q", after.feedback)
	}
	if !strings.Contains(after.View(), "六百円") {
		t.Fatalf("a miss should reveal the model answer:\n%s", after.View())
	}
	if cmd == nil {
		t.Fatal("revealing the model answer should play its audio")
	}
}

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

// Regression from review: the R6 skip gate was implemented but unreachable, so a
// learner could pass over any step with no demonstrations at all.
func TestSkipIsGatedOnTwoDifferentStimuli(t *testing.T) {
	model, store := newModelWith(t, readingUnits())

	updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	blocked := updated.(Model)
	if blocked.pos.step != model.pos.step {
		t.Fatal("a concept with no evidence must not be skippable")
	}
	if blocked.err == "" {
		t.Fatal("a refused skip must say why")
	}

	for i := 0; i < 2; i++ {
		if err := store.Record(profile.Evidence{
			ConceptID: "c1", Dimension: profile.Production, Correct: true,
			StimulusID: "same", Source: "typed", At: time.Now(),
		}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	updated, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if updated.(Model).pos.step != model.pos.step {
		t.Fatal("one repeated stimulus must not unlock skipping")
	}

	if err := store.Record(profile.Evidence{
		ConceptID: "c1", Dimension: profile.Production, Correct: true,
		StimulusID: "different", Source: "typed", At: time.Now(),
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	updated, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if updated.(Model).pos == model.pos && updated.(Model).screen != screenDone {
		t.Fatal("two different stimuli should unlock skipping")
	}
}

func TestProvisionalLLMEvidenceCannotUnlockSkipping(t *testing.T) {
	model, store := newModelWith(t, readingUnits())

	for _, stimulus := range []string{"one", "two"} {
		if err := store.Record(profile.Evidence{
			ConceptID: "c1", Dimension: profile.Production, Correct: true,
			StimulusID: stimulus, Source: "llm", At: time.Now(),
		}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if updated.(Model).pos.step != model.pos.step {
		t.Fatal("provisional LLM evidence must not clear the skip gate")
	}
}

func TestDueRetentionCheckIsAdministeredAndClearsMasteryGate(t *testing.T) {
	store, err := profile.Open(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	now := time.Now()
	for _, stimulus := range []string{"s1", "s2"} {
		if err := store.Record(profile.Evidence{
			ConceptID: "c1", Dimension: profile.Production, Correct: true,
			StimulusID: stimulus, Source: "typed", At: now,
		}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	if err := store.Record(profile.Evidence{
		ConceptID: "c1", Dimension: profile.Generalization, Correct: true,
		StimulusID: "s2", Source: "typed", At: now,
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := store.ScheduleReview(schedule.RetentionCard("c1", now.Add(-72*time.Hour))); err != nil {
		t.Fatalf("ScheduleReview: %v", err)
	}
	if mastered, _, _ := store.Mastered("c1"); mastered {
		t.Fatal("mastery must be blocked before the retention check")
	}

	model := New(&session.Runner{Store: store}, readingUnits())
	if model.retentionStep == nil {
		t.Fatal("a due retention check should be administered at session start")
	}
	if model.retentionConcept != "c1" {
		t.Fatalf("retention concept = %q, want c1", model.retentionConcept)
	}

	updated, _ := model.submit("mizu o kudasai")
	after := updated.(Model)
	if !after.correct {
		t.Fatalf("the retention answer should be correct, got %q", after.feedback)
	}

	mastered, missing, err := store.Mastered("c1")
	if err != nil {
		t.Fatalf("Mastered: %v", err)
	}
	if !mastered {
		t.Fatalf("passing the retention check should clear mastery, missing=%v", missing)
	}
	if due, _ := store.DueRetention(time.Now()); len(due) != 0 {
		t.Fatalf("the retention check should not stay due: %+v", due)
	}
}

func learnUnits() []*content.Unit {
	return []*content.Unit{{ID: "u", Title: "Unit", Taught: true, Lessons: []content.Lesson{{
		ID:        "l",
		Title:     "Requesting",
		Objective: "Ask for something with をください.",
		Anchors: []content.Text{
			{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"},
		},
		Teach: []content.Teach{{
			Prose: "Put をください after a noun to ask for it.",
			Examples: []content.Text{
				{Kanji: "お茶をください", Kana: "おちゃをください", Romaji: "ocha o kudasai"},
			},
		}},
		Steps: []content.Step{{
			ID: "s1", Kind: content.Construction, Concepts: []string{"c1"},
			Prompt: "Ask for water.", Accept: []content.Text{{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"}},
			Explanation: "をください makes the request.",
		}},
	}}}}
}

func TestLearnScreenShowsBeforeTheFirstStep(t *testing.T) {
	model, _ := newModelWith(t, learnUnits())

	if model.screen != screenLearn {
		t.Fatalf("a taught lesson with teaching material should open on the Learn screen, got %v", model.screen)
	}
	view := model.View()
	for _, want := range []string{
		"Ask for something with をください.",
		"水をください",
		"Put をください after a noun to ask for it.",
		"お茶をください",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("Learn screen missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Ask for water.") {
		t.Fatalf("the first step must not appear before the Learn screen is dismissed:\n%s", view)
	}

	updated, cmd := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("pressing an anchor number should play that anchor")
	}
	if model.spokenStep != "" {
		t.Fatal("playing an anchor must not mark the step as spoken")
	}

	updated, cmd = model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenPrompt {
		t.Fatalf("enter should leave the Learn screen, got %v", model.screen)
	}
	if !strings.Contains(model.View(), "Ask for water.") {
		t.Fatalf("the first step should appear after Learn:\n%s", model.View())
	}
}

func ladderUnits() []*content.Unit {
	step := func(id, prompt string) content.Step {
		return content.Step{
			ID: id, Kind: content.Construction, Concepts: []string{"c1"}, Prompt: prompt,
			Accept:      []content.Text{{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"}},
			Explanation: "をください makes the request.",
		}
	}
	hinted := step("s1", "Ask for water.")
	hinted.Hint = "Use をください after the noun."
	return []*content.Unit{{ID: "u", Title: "Unit", Lessons: []content.Lesson{{
		ID: "l", Title: "Lesson",
		Steps: []content.Step{hinted, step("s2", "Second."), step("s3", "Third."), step("s4", "Fourth.")},
	}}}}
}

func TestHintThenRevealThenRetry(t *testing.T) {
	model, store := newModelWith(t, ladderUnits())

	updated, _ := model.submit("wrong")
	model = updated.(Model)
	if model.screen != screenPrompt {
		t.Fatalf("a first miss with a hint should stay on the prompt, got %v", model.screen)
	}
	if !strings.Contains(model.View(), "Use をください after the noun.") {
		t.Fatalf("the hint should be visible:\n%s", model.View())
	}
	if model.pos.step != 0 {
		t.Fatalf("the learner should still be on the same step, got %d", model.pos.step)
	}
	if errors, _ := store.Errors("c1"); len(errors) != 0 {
		t.Fatalf("a hint rung must not record a wrong attempt, got %+v", errors)
	}

	updated, cmd := model.submit("still wrong")
	model = updated.(Model)
	if model.screen != screenFeedback {
		t.Fatalf("a second miss should reveal the answer, got %v", model.screen)
	}
	if !strings.Contains(model.View(), "水をください") {
		t.Fatalf("the model answer should be written out:\n%s", model.View())
	}
	if !strings.Contains(model.View(), "をください makes the request.") {
		t.Fatalf("the explanation should be shown with the answer:\n%s", model.View())
	}
	if cmd == nil {
		t.Fatal("revealing the answer should play its audio")
	}
	if errors, _ := store.Errors("c1"); len(errors) != 0 {
		t.Fatalf("the reveal rung must not record a wrong attempt, got %+v", errors)
	}

	for i := 0; i < 3; i++ {
		model.advance()
	}
	if model.pos.step != 3 {
		t.Fatalf("three other steps should run before the retry, got step %d", model.pos.step)
	}
	model.advance()
	if model.pos.step != 4 {
		t.Fatalf("the retry should be queued at least three positions later, got step %d", model.pos.step)
	}
	_, lesson, step, ok := model.current()
	if !ok || step.ID != "s1" || lesson.ID != "l" {
		t.Fatalf("expected the s1 retry, got lesson %v step %v", lesson, step)
	}

	updated, _ = model.submit("mizu o kudasai")
	model = updated.(Model)
	if !model.correct {
		t.Fatalf("the second-chance answer should be correct, got %q", model.feedback)
	}
	evidence, err := store.Evidence("c1")
	if err != nil || len(evidence) != 1 || evidence[0].Source != "guided" {
		t.Fatalf("the second-chance correct should record one guided attempt, got %+v err=%v", evidence, err)
	}
	if can, _ := store.CanSkip("c1"); can {
		t.Fatal("a guided attempt must not unlock skipping")
	}
	if mastered, _, _ := store.Mastered("c1"); mastered {
		t.Fatal("a guided attempt must not satisfy mastery")
	}
	cards, _ := store.Reviews()
	for _, card := range cards {
		if card.Kind == schedule.KindRetention {
			t.Fatalf("a guided attempt must not schedule retention, got %+v", card)
		}
	}
}

func TestNoHintGoesStraightToTheReveal(t *testing.T) {
	units := ladderUnits()
	units[0].Lessons[0].Steps[0].Hint = ""
	model, _ := newModelWith(t, units)

	updated, cmd := model.submit("wrong")
	model = updated.(Model)
	if model.screen != screenFeedback {
		t.Fatalf("without a hint the first miss should reveal the answer, got %v", model.screen)
	}
	if !strings.Contains(model.View(), "水をください") {
		t.Fatalf("the model answer should be written out:\n%s", model.View())
	}
	if cmd == nil {
		t.Fatal("revealing the answer should play its audio")
	}
}

func TestThirdMissRecordsAnOrdinaryWrongAttempt(t *testing.T) {
	model, store := newModelWith(t, ladderUnits())

	updated, _ := model.submit("wrong")
	model = updated.(Model)
	updated, _ = model.submit("still wrong")
	model = updated.(Model)
	for i := 0; i < 4; i++ {
		model.advance()
	}

	updated, _ = model.submit("wrong again")
	model = updated.(Model)
	if model.correct {
		t.Fatal("a third miss is still a miss")
	}
	errors, _ := store.Errors("c1")
	if len(errors) != 1 {
		t.Fatalf("a third miss should record exactly one ordinary wrong attempt, got %+v", errors)
	}
	if evidence, _ := store.Evidence("c1"); len(evidence) != 0 {
		t.Fatalf("a third miss must not mint evidence, got %+v", evidence)
	}
}

func TestRetentionStepDoesNotClobberTheResumedCursor(t *testing.T) {
	store, err := profile.Open(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	units := ladderUnits()
	if err := store.SaveCursor("u", "l", 2, time.Now()); err != nil {
		t.Fatalf("SaveCursor: %v", err)
	}
	if err := store.ScheduleReview(schedule.RetentionCard("c1", time.Now().Add(-72*time.Hour))); err != nil {
		t.Fatalf("ScheduleReview: %v", err)
	}

	model := New(&session.Runner{Store: store}, units)
	if model.pos.step != 2 || model.retentionStep == nil {
		t.Fatalf("expected to resume at step 2 with a retention check, pos=%d retention=%v", model.pos.step, model.retentionStep)
	}
	updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	_ = updated.(Model).View()

	rebuilt := New(&session.Runner{Store: store}, units)
	if rebuilt.pos.step != 2 {
		t.Fatalf("a retention step must not overwrite the saved lesson cursor, resumed at %d", rebuilt.pos.step)
	}
}

func TestDenialCounterResetsPerStep(t *testing.T) {
	units := speakingUnits()
	units[0].Lessons[0].Steps = append(units[0].Lessons[0].Steps, content.Step{
		ID: "s2", Kind: content.Speaking, Concepts: []string{"c1"},
		Prompt: "Say it again.", Accept: []content.Text{{Kana: "みず", Romaji: "mizu"}}, Explanation: "because",
	})
	model, _ := newModelWith(t, units)

	model.screen = screenConfirm
	model.transcript = "nope"
	updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model = updated.(Model)
	model.screen = screenConfirm
	model.transcript = "nope"
	updated, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model = updated.(Model)
	if !model.typedFallback {
		t.Fatal("two denials on one step should offer the typed fallback")
	}

	model.advance()
	model.screen = screenConfirm
	model.transcript = "nope"
	updated, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model = updated.(Model)
	if model.typedFallback {
		t.Fatal("the first denial on a new step must not offer the fallback")
	}
}
