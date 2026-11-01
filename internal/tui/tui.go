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
	screenFeedback
	screenConfirm
	screenDone
)

type position struct {
	unit, lesson, step int
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
	retentionStep    *content.Step
	retentionConcept string
	spokenStep       string
	err              string
}

func New(runner *session.Runner, units []*content.Unit) Model {
	input := textinput.New()
	input.Placeholder = "romaji, kana or kanji"
	input.Focus()

	m := Model{runner: runner, units: units, input: input}

	if unitID, lessonID, step, ok, err := runner.Store.Cursor(); err == nil && ok {
		if resumed, found := findPosition(units, unitID, lessonID, step); found {
			m.pos = resumed
		}
	}
	m.syncConversation()
	m.startRetention()
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

func (m Model) Init() tea.Cmd { return m.playCurrent() }

func (m Model) current() (*content.Unit, *content.Lesson, *content.Step, bool) {
	if m.retentionStep != nil && m.pos.unit < len(m.units) {
		unit := m.units[m.pos.unit]
		return unit, &content.Lesson{ID: "retention", Title: "Retention check"}, m.retentionStep, true
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
	if step.Kind != content.Listening || step.Target == nil {
		return ""
	}
	if step.Target.Kanji != "" {
		return step.Target.Kanji
	}
	return step.Target.Kana
}

func (m *Model) playCurrent() tea.Cmd {
	unit, _, step, ok := m.current()
	if !ok {
		return nil
	}
	_ = unit
	text := audioText(step)
	if text == "" || m.spokenStep == step.ID {
		return nil
	}
	m.spokenStep = step.ID

	client := m.runner.Client
	player := m.player
	voice := m.runner.Voice
	stepID := step.ID

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
	}

	unit, lesson, step, ok := m.current()
	if !ok {
		m.screen = screenDone
		return m, nil
	}
	if err := m.runner.Store.SaveCursor(unit.ID, lesson.ID, m.pos.step, m.runner.Clock()); err != nil {
		m.err = err.Error()
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

func (m *Model) startRetention() {
	due, err := m.runner.DueRetention()
	if err != nil || len(due) == 0 {
		return
	}
	for _, card := range due {
		if step := findRetentionStep(m.units, card.ConceptID); step != nil {
			m.retentionConcept = card.ConceptID
			m.retentionStep = step
			m.screen = screenPrompt
			return
		}
	}
}

func findRetentionStep(units []*content.Unit, conceptID string) *content.Step {
	var found *content.Step
	for _, unit := range units {
		for li := range unit.Lessons {
			for si := range unit.Lessons[li].Steps {
				step := &unit.Lessons[li].Steps[si]
				if step.Rubric != "" || step.Scenario != nil || content.Dimension(step.Kind) == "" {
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

	var result session.Result
	var err error

	if m.retentionStep != nil {
		result, err = m.runner.GradeRetention(step, answer)
	} else if step.Kind == content.Conversation {
		if m.conversation == nil {
			m.syncConversation()
		}
		if m.conversation == nil {
			m.err = "this conversation has no scenario"
			return m, nil
		}
		response := m.conversation.Respond(answer)
		result, err = m.runner.GradeConversationTurn(step, response)
		if err == nil && response.Verdict != convo.NoMatch {
			m.conversation.Advance()
			m.collectNpcTurns()
		}
	} else {
		result, err = m.runner.Submit(context.Background(), step, answer)
	}
	if err != nil {
		m.err = err.Error()
		return m, nil
	}

	m.feedback = result.Outcome.Explanation
	m.correct = result.Outcome.Correct
	m.screen = screenFeedback
	return m, nil
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
	if m.retentionStep != nil {
		m.retentionStep = nil
		m.retentionConcept = ""
		m.screen = screenPrompt
		m.feedback = ""
		m.transcript = ""
		m.err = ""
		m.input.SetValue("")
		return
	}
	m.screen = screenPrompt
	m.feedback = ""
	m.transcript = ""
	m.spokenStep = ""
	m.degraded = false
	m.syncConversation()
	m.typedFallback = false
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
		return
	}
	m.pos.lesson = 0
	m.pos.unit++
	if m.pos.unit >= len(m.units) {
		m.screen = screenDone
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
	case screenConfirm:
		b.WriteString(titleStyle.Render("Here is what I heard — correct?") + "\n\n")
		b.WriteString(japaneseStyle.Render(m.transcript) + "\n\n")
		if m.ambiguous {
			b.WriteString(badStyle.Render("Low confidence — check every particle before confirming.") + "\n\n")
		}
		b.WriteString(dimStyle.Render("This transcript is what gets graded. y to confirm, n to say it again.") + "\n")
		return b.String()

	case screenFeedback:
		if m.correct {
			b.WriteString(goodStyle.Render("Correct") + "\n\n")
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
