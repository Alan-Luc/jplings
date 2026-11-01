package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jplings/internal/audio"
	"jplings/internal/content"
	"jplings/internal/convo"
	"jplings/internal/grade"
	"jplings/internal/schedule"
	"jplings/internal/session"
	"jplings/internal/speech"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	dimStyle      = lipgloss.NewStyle().Faint(true)
	goodStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	badStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	japaneseStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

type screen int

const (
	screenPrompt screen = iota
	screenLearn
	screenFeedback
	screenConfirm
	screenGrading
	screenDone
)

type position struct {
	unit, lesson, step int
}

type reviewItem struct {
	card schedule.Card
	step *content.Step
}

type spokenMsg struct {
	stepID   string
	degraded bool
	err      error
}

type recordResult struct {
	wav []byte
	err error
}

type gradedMsg struct {
	result session.Result
	step   *content.Step
	err    error
}

type Model struct {
	runner   *session.Runner
	units    []*content.Unit
	pos      position
	input    textinput.Model
	screen   screen
	feedback string
	correct  bool

	transcript       string
	ambiguous        bool
	turn             speech.Turn
	recorder         audio.Recorder
	player           audio.Player
	recording        bool
	cancelRecord     context.CancelFunc
	degraded         bool
	typedFallback    bool
	conversation     *convo.Conversation
	conversationStep string
	npcLines         []content.Text
	reviews          []reviewItem
	passed           map[string]bool
	spokenStep       string
	hint             string
	reveal           *content.Text
	misses           map[string]int
	revealed         map[string]bool
	requeueStep      *content.Step
	err              string
}

func New(runner *session.Runner, units []*content.Unit) Model {
	input := textinput.New()
	input.Placeholder = "romaji, kana or kanji"
	input.Focus()

	m := Model{runner: runner, units: units, input: input, passed: map[string]bool{}}
	m.ensureRetryState()

	if unitID, lessonID, step, ok, err := runner.Store.Cursor(); err == nil && ok {
		if resumed, found := findPosition(units, unitID, lessonID, step); found {
			m.pos = resumed
		}
	}
	m.syncConversation()
	m.startReviews()
	if err := runner.RetryPending(context.Background(), func(id string) *content.Step {
		return findStepByID(units, id)
	}); err != nil {
		m.err = err.Error()
	}
	if len(m.reviews) == 0 {
		if unit, lesson, _, ok := m.current(); ok && m.pos.step == 0 && learnAvailable(unit, lesson) {
			m.screen = screenLearn
		}
	}
	return m
}

func findPosition(units []*content.Unit, unitID, lessonID string, step int) (position, bool) {
	for u := range units {
		if units[u].ID != unitID {
			continue
		}
		for l := range units[u].Lessons {
			if units[u].Lessons[l].ID != lessonID {
				continue
			}
			if step >= 0 && step < len(units[u].Lessons[l].Steps) {
				return position{unit: u, lesson: l, step: step}, true
			}
		}
	}
	return position{}, false
}

func (m Model) Init() tea.Cmd {
	if m.screen == screenLearn {
		return nil
	}
	return m.playCurrent()
}

func (m Model) current() (*content.Unit, *content.Lesson, *content.Step, bool) {
	if len(m.reviews) > 0 && m.pos.unit < len(m.units) {
		unit := m.units[m.pos.unit]
		return unit, &content.Lesson{ID: "review", Title: "Review"}, m.reviews[0].step, true
	}
	if m.pos.unit >= len(m.units) {
		return nil, nil, nil, false
	}
	unit := m.units[m.pos.unit]
	if m.pos.lesson >= len(unit.Lessons) {
		return nil, nil, nil, false
	}
	lesson := &unit.Lessons[m.pos.lesson]
	if m.pos.step >= len(lesson.Steps) {
		return nil, nil, nil, false
	}
	return unit, lesson, &lesson.Steps[m.pos.step], true
}

func audioText(step *content.Step) string {
	if step.Target == nil {
		return ""
	}
	if step.Kind != content.Listening && len(step.AcceptMeanings) == 0 {
		return ""
	}
	return surface(*step.Target)
}

func (m *Model) playCurrent() tea.Cmd {
	_, _, step, ok := m.current()
	if !ok {
		return nil
	}
	text := audioText(step)
	if text == "" || m.spokenStep == step.ID {
		return nil
	}
	m.spokenStep = step.ID
	return m.speak(step.ID, text)
}

func (m *Model) playAcceptedAnswer(step *content.Step) tea.Cmd {
	if len(step.Accept) == 0 || content.Dimension(step.Kind) != "production" {
		return nil
	}
	return m.speak(step.ID, surface(step.Accept[0]))
}

func (m *Model) playModelAnswer(step *content.Step) tea.Cmd {
	answer := modelAnswer(step)
	if answer == nil {
		return nil
	}
	return m.speak(step.ID, surface(*answer))
}

func modelAnswer(step *content.Step) *content.Text {
	var answer *content.Text
	if len(step.Accept) > 0 {
		answer = &step.Accept[0]
	} else {
		answer = step.Target
	}
	if answer == nil {
		return nil
	}
	copied := *answer
	return &copied
}

func learnAvailable(unit *content.Unit, lesson *content.Lesson) bool {
	return unit.Taught && (lesson.Objective != "" || len(lesson.Anchors) > 0 || len(lesson.Teach) > 0)
}

func (m *Model) ensureRetryState() {
	if m.misses == nil {
		m.misses = map[string]int{}
	}
	if m.revealed == nil {
		m.revealed = map[string]bool{}
	}
}

func anchorIndex(msg tea.KeyMsg) (int, bool) {
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		return 0, false
	}
	r := msg.Runes[0]
	if r < '1' || r > '9' {
		return 0, false
	}
	return int(r - '1'), true
}

func (m *Model) speak(stepID, text string) tea.Cmd {
	client, player, voice := m.runner.Client, m.player, m.runner.Voice
	if client == nil {
		return func() tea.Msg { return spokenMsg{stepID: stepID, degraded: true} }
	}
	return func() tea.Msg {
		wav, err := client.Synthesize(context.Background(), text, "ja", voice)
		if err != nil {
			return spokenMsg{stepID: stepID, degraded: true, err: err}
		}
		if err := player.Play(wav); err != nil {
			return spokenMsg{stepID: stepID, degraded: true, err: err}
		}
		return spokenMsg{stepID: stepID}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spokenMsg:
		m.degraded = msg.degraded
		if msg.err != nil {
			m.err = "Audio unavailable — showing the written prompt. " + msg.err.Error()
		}
		return m, nil

	case recordResult:
		m.recording = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		return m, m.transcribe(msg.wav)

	case gradedMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.screen = screenPrompt
			return m, nil
		}
		return m.showOutcome(msg.result, m.playAcceptedAnswer(msg.step))

	case speech.Transcription:
		m.transcript = strings.TrimSpace(msg.Text)
		m.ambiguous = msg.Ambiguous
		m.screen = screenConfirm
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.screen {
	case screenLearn:
		if msg.String() == "enter" {
			m.screen = screenPrompt
			return m, m.playCurrent()
		}
		if index, ok := anchorIndex(msg); ok {
			_, lesson, _, has := m.current()
			if has && index < len(lesson.Anchors) {
				return m, m.speak("anchor", surface(lesson.Anchors[index]))
			}
		}
		return m, nil

	case screenConfirm:
		switch strings.ToLower(msg.String()) {
		case "y":
			if decision := m.turn.Confirm(); decision.Grade {
				return m.submit(m.transcript)
			}
		case "n":
			decision := m.turn.Deny()
			m.transcript = ""
			if decision.OfferFallback {
				m.screen = screenPrompt
				m.typedFallback = true
				m.feedback = "Transcription denied twice. Type your answer instead, or press Esc to move on without evidence."
				m.err = ""
				m.input.Focus()
				return m, nil
			}
			m.screen = screenPrompt
			m.feedback = "Denied — say it again. Nothing was recorded."
			return m, nil
		}
		return m, nil

	case screenFeedback:
		if msg.String() == "enter" {
			m.advance()
			return m, m.playCurrent()
		}
		return m, nil

	case screenDone:
		if msg.String() == "q" || msg.String() == "enter" {
			return m, tea.Quit
		}
		return m, nil

	case screenGrading:
		return m, nil
	}

	unit, lesson, step, ok := m.current()
	if !ok {
		m.screen = screenDone
		return m, nil
	}
	if len(m.reviews) == 0 {
		if err := m.runner.Store.SaveCursor(unit.ID, lesson.ID, m.pos.step, m.runner.Clock()); err != nil {
			m.err = err.Error()
		}
	}

	if msg.String() == "esc" {
		m.advance()
		return m, m.playCurrent()
	}

	if needsSpeech(step) {
		if m.typedFallback {
			if msg.String() == "enter" {
				answer := strings.TrimSpace(m.input.Value())
				if answer == "" {
					return m, nil
				}
				m.input.SetValue("")
				return m.submit(answer)
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		switch {
		case msg.String() == "r":
			if m.recording {
				return m, nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			m.cancelRecord = cancel
			m.recording = true
			m.err = ""
			recorder := m.recorder
			return m, func() tea.Msg {
				wav, err := recorder.Record(ctx)
				return recordResult{wav: wav, err: err}
			}
		case isSpace(msg):
			if m.recording && m.cancelRecord != nil {
				m.cancelRecord()
				m.cancelRecord = nil
			}
			return m, nil
		}
		return m, nil
	}

	if msg.String() == "ctrl+s" {
		return m.skipCurrent()
	}

	if msg.String() == "ctrl+r" && audioText(step) != "" {
		m.spokenStep = ""
		return m, m.playCurrent()
	}

	if msg.String() == "enter" {
		answer := strings.TrimSpace(m.input.Value())
		if answer == "" {
			return m, nil
		}
		m.input.SetValue("")
		return m.submit(answer)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func isSpace(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeySpace || msg.String() == " " || msg.String() == "space"
}

func needsSpeech(step *content.Step) bool {
	return step.Kind == content.Speaking || step.Kind == content.Conversation
}

func (m *Model) startReviews() {
	due, err := m.runner.DueCards()
	if err != nil {
		m.err = err.Error()
		return
	}
	for _, card := range due {
		step := siblingStep(m.units, card.ConceptID, card.StepID)
		if step == nil || m.passed[step.ID] {
			if own := findStepByID(m.units, card.StepID); own != nil && eligibleReviewStep(own) {
				step = own
			}
		}
		if step == nil || m.passed[step.ID] {
			continue
		}
		m.reviews = append(m.reviews, reviewItem{card: card, step: step})
	}
	if len(m.reviews) > 0 {
		m.screen = screenPrompt
	}
}

func eligibleReviewStep(step *content.Step) bool {
	return step.Rubric == "" && step.Scenario == nil && content.Dimension(step.Kind) != ""
}

func findStepByID(units []*content.Unit, stepID string) *content.Step {
	for _, unit := range units {
		for li := range unit.Lessons {
			for si := range unit.Lessons[li].Steps {
				step := &unit.Lessons[li].Steps[si]
				if step.ID == stepID {
					return step
				}
			}
		}
	}
	return nil
}

func siblingStep(units []*content.Unit, conceptID, excludeID string) *content.Step {
	var found *content.Step
	for _, unit := range units {
		for li := range unit.Lessons {
			for si := range unit.Lessons[li].Steps {
				step := &unit.Lessons[li].Steps[si]
				if step.ID == excludeID || !eligibleReviewStep(step) {
					continue
				}
				for _, concept := range step.Concepts {
					if concept == conceptID {
						found = step
					}
				}
			}
		}
	}
	return found
}

func (m *Model) syncConversation() {
	_, _, step, ok := m.current()
	if !ok || step.Kind != content.Conversation || step.Scenario == nil {
		m.conversation = nil
		m.conversationStep = ""
		m.npcLines = nil
		return
	}
	if m.conversationStep == step.ID && m.conversation != nil {
		return
	}
	m.conversation = convo.New(step.Scenario)
	m.conversationStep = step.ID
	m.npcLines = nil
	m.collectNpcTurns()
}

func (m *Model) collectNpcTurns() {
	if m.conversation == nil {
		return
	}
	for !m.conversation.Done() {
		turn, ok := m.conversation.Current()
		if !ok || turn.Speaker != "npc" {
			return
		}
		if turn.Line != nil {
			m.npcLines = append(m.npcLines, *turn.Line)
		}
		m.conversation.Advance()
	}
}

func (m Model) skipCurrent() (tea.Model, tea.Cmd) {
	_, _, step, ok := m.current()
	if !ok {
		return m, nil
	}
	for _, conceptID := range step.Concepts {
		can, err := m.runner.MaySkip(conceptID)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		if !can {
			m.err = fmt.Sprintf(
				"Cannot skip %s yet: skipping needs two production demonstrations on different exercises.", conceptID)
			return m, nil
		}
	}
	m.err = ""
	m.advance()
	return m, m.playCurrent()
}

func (m Model) submit(answer string) (tea.Model, tea.Cmd) {
	_, _, step, ok := m.current()
	if !ok {
		m.screen = screenDone
		return m, nil
	}
	m.ensureRetryState()

	if len(m.reviews) > 0 {
		item := m.reviews[0]
		result, err := m.runner.Review(item.card, item.step, answer, m.passed[item.step.ID])
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.passed[item.step.ID] = true
		return m.showOutcome(result, m.playAcceptedAnswer(item.step))
	}
	m.passed[step.ID] = true

	if step.Kind == content.Conversation {
		if m.conversation == nil {
			m.syncConversation()
		}
		if m.conversation == nil {
			m.err = "this conversation has no scenario"
			return m, nil
		}
		response := m.conversation.Respond(answer)
		result, err := m.runner.GradeConversationTurn(step, response)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		if response.Verdict != convo.NoMatch {
			m.conversation.Advance()
			m.collectNpcTurns()
		}
		return m.showOutcome(result, m.playAcceptedAnswer(step))
	}

	if step.Kind == content.Speaking && step.Rubric != "" {
		m.feedback = "Saved for grading — the AI verdict will appear here."
		m.correct = false
		m.err = ""
		m.reveal = nil
		m.screen = screenGrading
		runner := m.runner
		return m, func() tea.Msg {
			result, err := runner.Submit(context.Background(), step, answer)
			return gradedMsg{result: result, step: step, err: err}
		}
	}

	outcome := m.runner.Grade(step, answer)
	if !outcome.Correct {
		return m.handleMiss(step, answer, outcome)
	}

	var result session.Result
	var err error
	if m.revealed[step.ID] {
		result, err = m.runner.SubmitGuided(context.Background(), step, answer)
	} else {
		result, err = m.runner.Submit(context.Background(), step, answer)
	}
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	return m.showOutcome(result, nil)
}

func (m Model) showOutcome(result session.Result, missCmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.feedback = result.Outcome.Explanation
	m.correct = result.Outcome.Correct
	m.screen = screenFeedback
	if m.correct {
		return m, nil
	}
	return m, missCmd
}

func (m Model) handleMiss(step *content.Step, answer string, outcome grade.Outcome) (tea.Model, tea.Cmd) {
	if m.revealed[step.ID] {
		result, err := m.runner.Submit(context.Background(), step, answer)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		return m.showOutcome(result, m.playAcceptedAnswer(step))
	}

	m.misses[step.ID]++
	if m.misses[step.ID] == 1 && step.Hint != "" {
		m.hint = step.Hint
		m.feedback = ""
		m.correct = false
		m.screen = screenPrompt
		m.input.SetValue("")
		m.err = ""
		return m, nil
	}

	m.hint = ""
	m.revealed[step.ID] = true
	retry := *step
	m.requeueStep = &retry
	m.feedback = outcome.Explanation
	m.correct = false
	m.reveal = modelAnswer(step)
	m.screen = screenFeedback
	return m, m.playModelAnswer(step)
}

func nextStepPosition(units []*content.Unit, pos position) (position, bool) {
	if pos.unit >= len(units) || pos.lesson >= len(units[pos.unit].Lessons) {
		return position{}, false
	}
	lesson := units[pos.unit].Lessons[pos.lesson]
	if pos.step+1 < len(lesson.Steps) {
		return position{unit: pos.unit, lesson: pos.lesson, step: pos.step + 1}, true
	}
	for l := pos.lesson + 1; l < len(units[pos.unit].Lessons); l++ {
		if len(units[pos.unit].Lessons[l].Steps) > 0 {
			return position{unit: pos.unit, lesson: l}, true
		}
	}
	for u := pos.unit + 1; u < len(units); u++ {
		for l := range units[u].Lessons {
			if len(units[u].Lessons[l].Steps) > 0 {
				return position{unit: u, lesson: l}, true
			}
		}
	}
	return position{}, false
}

func (m *Model) insertRetry(step content.Step) {
	pos := m.pos
	for i := 0; i < 3; i++ {
		next, ok := nextStepPosition(m.units, pos)
		if !ok {
			m.appendRetry(step)
			return
		}
		pos = next
	}

	lesson := &m.units[pos.unit].Lessons[pos.lesson]
	index := pos.step + 1
	lesson.Steps = append(lesson.Steps, content.Step{})
	copy(lesson.Steps[index+1:], lesson.Steps[index:])
	lesson.Steps[index] = step
}

func (m *Model) appendRetry(step content.Step) {
	for u := len(m.units) - 1; u >= 0; u-- {
		if len(m.units[u].Lessons) == 0 {
			continue
		}
		l := len(m.units[u].Lessons) - 1
		m.units[u].Lessons[l].Steps = append(m.units[u].Lessons[l].Steps, step)
		return
	}
}

func (m Model) transcribe(wav []byte) tea.Cmd {
	client := m.runner.Client
	return func() tea.Msg {
		if hasSpeech, _, err := speech.HasSpeech(wav, speech.SilenceFloor); err == nil && !hasSpeech {
			return speech.Transcription{Text: "", Ambiguous: true}
		}
		if client == nil {
			return speech.Transcription{}
		}
		got, err := client.Transcribe(context.Background(), wav, "ja")
		if err != nil {
			return speech.Transcription{}
		}
		return got
	}
}

func (m *Model) advance() {
	m.turn = speech.Turn{}
	if m.requeueStep != nil {
		m.insertRetry(*m.requeueStep)
		m.requeueStep = nil
	}

	if len(m.reviews) > 0 {
		m.reviews = m.reviews[1:]
		m.screen = screenPrompt
		m.feedback = ""
		m.transcript = ""
		m.err = ""
		m.input.SetValue("")
		if len(m.reviews) > 0 {
			return
		}
		if unit, lesson, _, ok := m.current(); ok && m.pos.step == 0 && learnAvailable(unit, lesson) {
			m.screen = screenLearn
		}
		return
	}
	m.screen = screenPrompt
	m.feedback = ""
	m.transcript = ""
	m.spokenStep = ""
	m.degraded = false
	m.syncConversation()
	m.typedFallback = false
	m.hint = ""
	m.reveal = nil
	m.err = ""
	m.input.SetValue("")

	unit, lesson, _, ok := m.current()
	if !ok {
		m.screen = screenDone
		return
	}

	m.pos.step++
	if m.pos.step < len(lesson.Steps) {
		return
	}
	m.pos.step = 0
	m.pos.lesson++
	if m.pos.lesson < len(unit.Lessons) {
		if nextUnit, nextLesson, _, found := m.current(); found && learnAvailable(nextUnit, nextLesson) {
			m.screen = screenLearn
		}
		return
	}
	m.pos.lesson = 0
	m.pos.unit++
	if m.pos.unit >= len(m.units) {
		m.screen = screenDone
		return
	}
	if nextUnit, nextLesson, _, found := m.current(); found && learnAvailable(nextUnit, nextLesson) {
		m.screen = screenLearn
	}
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(dimStyle.Render(fmt.Sprintf("jplings  ·  unit %d/%d", m.pos.unit+1, len(m.units))) + "\n\n")

	unit, lesson, step, ok := m.current()
	if !ok || m.screen == screenDone {
		b.WriteString(titleStyle.Render("Session complete") + "\n\n")
		b.WriteString("press q to quit\n")
		return b.String()
	}

	b.WriteString(dimStyle.Render(unit.Title+"  ·  "+lesson.Title) + "\n\n")

	switch m.screen {
	case screenLearn:
		b.WriteString(titleStyle.Render("Learn") + "\n\n")
		if lesson.Objective != "" {
			b.WriteString(lesson.Objective + "\n\n")
		}
		if len(lesson.Anchors) > 0 {
			b.WriteString(titleStyle.Render("You already know") + "\n")
			for i, anchor := range lesson.Anchors {
				b.WriteString(fmt.Sprintf("%d. %s", i+1, surface(anchor)) + "\n")
				if anchor.Kana != "" || anchor.Romaji != "" {
					b.WriteString(dimStyle.Render(anchor.Kana+"  "+anchor.Romaji) + "\n")
				}
			}
			b.WriteString("\n")
		}
		for _, item := range lesson.Teach {
			b.WriteString(item.Prose + "\n\n")
			for _, example := range item.Examples {
				b.WriteString(japaneseStyle.Render(surface(example)) + "\n")
				if example.Kana != "" || example.Romaji != "" {
					b.WriteString(dimStyle.Render(example.Kana+"  "+example.Romaji) + "\n")
				}
			}
			if len(item.Examples) > 0 {
				b.WriteString("\n")
			}
		}
		b.WriteString(dimStyle.Render("number to hear an anchor · enter to start") + "\n")
		return b.String()

	case screenConfirm:
		b.WriteString(titleStyle.Render("Here is what I heard — correct?") + "\n\n")
		b.WriteString(japaneseStyle.Render(m.transcript) + "\n\n")
		if m.ambiguous {
			b.WriteString(badStyle.Render("Low confidence — check every particle before confirming.") + "\n\n")
		}
		b.WriteString(dimStyle.Render("This transcript is what gets graded. y to confirm, n to say it again.") + "\n")
		return b.String()

	case screenGrading:
		b.WriteString(titleStyle.Render("Grading") + "\n\n")
		b.WriteString(m.feedback + "\n\n")
		b.WriteString(dimStyle.Render("the verdict will appear here") + "\n")
		return b.String()

	case screenFeedback:
		if m.correct {
			b.WriteString(goodStyle.Render("Correct") + "\n\n")
		} else if m.reveal != nil {
			b.WriteString(badStyle.Render("Not quite — here is the model answer") + "\n\n")
			b.WriteString(japaneseStyle.Render(surface(*m.reveal)) + "\n")
			if m.reveal.Kana != "" || m.reveal.Romaji != "" {
				b.WriteString(dimStyle.Render(m.reveal.Kana+"  "+m.reveal.Romaji) + "\n")
			}
			b.WriteString("\n" + m.feedback + "\n\n")
			b.WriteString(dimStyle.Render("enter to continue — this step returns later in the session") + "\n")
			return b.String()
		} else {
			b.WriteString(badStyle.Render("Not quite") + "\n\n")
		}
		b.WriteString(m.feedback + "\n\n")
		b.WriteString(dimStyle.Render("enter to continue") + "\n")
		return b.String()
	}

	b.WriteString(titleStyle.Render(strings.ToUpper(string(step.Kind))) + "\n")
	for _, line := range m.npcLines {
		b.WriteString(dimStyle.Render(surface(line)) + "\n")
	}
	if len(m.npcLines) > 0 {
		b.WriteString("\n")
	}
	b.WriteString(step.Prompt + "\n\n")

	if step.Target != nil && step.Kind != content.Listening {
		b.WriteString(japaneseStyle.Render(surface(*step.Target)) + "\n")
		b.WriteString(dimStyle.Render(step.Target.Romaji) + "\n\n")
	}
	if step.Kind == content.Listening {
		if m.degraded {
			b.WriteString(badStyle.Render("audio unavailable") + dimStyle.Render(" — written prompt follows") + "\n")
			if step.Target != nil {
				b.WriteString(japaneseStyle.Render(surface(*step.Target)) + "\n")
			}
			b.WriteString("\n")
		} else {
			b.WriteString(dimStyle.Render("listen — ctrl+r to replay") + "\n\n")
		}
	}

	if m.hint != "" {
		b.WriteString(badStyle.Render("Hint: "+m.hint) + "\n\n")
	}

	if needsSpeech(step) {
		if m.typedFallback {
			if m.feedback != "" {
				b.WriteString(badStyle.Render(m.feedback) + "\n\n")
			}
			b.WriteString(m.input.View() + "\n\n")
			b.WriteString(dimStyle.Render("enter to answer · esc to skip without evidence") + "\n")
			return b.String()
		}
		switch {
		case m.recording:
			b.WriteString(badStyle.Render("● recording") + dimStyle.Render(" — press SPACE to stop") + "\n")
		case m.err != "":
			b.WriteString(badStyle.Render(m.err) + "\n")
			b.WriteString(dimStyle.Render("r to record, esc to skip") + "\n")
		default:
			b.WriteString(dimStyle.Render("r to record, then SPACE to stop · esc to skip") + "\n")
		}
		return b.String()
	}

	if m.err != "" {
		b.WriteString(badStyle.Render(m.err) + "\n\n")
	}
	b.WriteString(m.input.View() + "\n\n")
	b.WriteString(dimStyle.Render("enter to answer · esc to skip · ctrl+c to quit") + "\n")
	return b.String()
}

func surface(t content.Text) string {
	if t.Kanji != "" {
		return t.Kanji
	}
	return t.Kana
}

func Run(runner *session.Runner, units []*content.Unit) error {
	_, err := tea.NewProgram(New(runner, units), tea.WithAltScreen()).Run()
	return err
}
