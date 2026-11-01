// Package tui is the Bubble Tea interface (D1).
//
// The screens are deliberately plain: a prompt, the exercise, feedback that names
// what went wrong, and for speech the confirmation step that stands between a
// transcript and a grade.
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

// position is where the learner is in the content tree.
type position struct {
	unit, lesson, step int
}

type Model struct {
	runner   *session.Runner
	units    []*content.Unit
	pos      position
	input    textinput.Model
	screen   screen
	feedback string
	correct  bool

	// Speech state for the confirmation step (R4).
	transcript   string
	ambiguous    bool
	turn         speech.Turn
	recorder     audio.Recorder
	player       audio.Player
	recording    bool
	cancelRecord context.CancelFunc
	pendingWav   []byte
	degraded     bool
	err          string
}

// New builds the session model, resuming from the stored cursor when possible.
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

func (m Model) Init() tea.Cmd { return nil }

func (m Model) current() (*content.Unit, *content.Lesson, *content.Step, bool) {
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

type recordResult struct {
	wav []byte
	err error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case recordResult:
		m.recording = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.pendingWav = msg.wav
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
			decision := m.turn.Confirm()
			if decision.Grade {
				return m.submit(m.transcript)
			}
		case "n":
			decision := m.turn.Deny()
			m.pendingWav = nil
			m.transcript = ""
			if decision.OfferFallback {
				// Two consecutive denials: offer typed romaji or a no-evidence advance
				// rather than deadlocking progress.
				m.screen = screenPrompt
				m.feedback = "Transcription denied twice. Type your answer instead, or press Esc to move on without evidence."
				m.err = ""
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
			return m, nil
		}
		return m, nil

	case screenDone:
		if msg.String() == "q" || msg.String() == "enter" {
			return m, tea.Quit
		}
		return m, nil
	}

	// screenPrompt
	if msg.String() == "esc" {
		m.advance()
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

	if needsSpeech(step) {
		switch msg.String() {
		case "r":
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
		case "space":
			if m.recording && m.cancelRecord != nil {
				m.cancelRecord()
			}
			return m, nil
		}
		return m, nil
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

func needsSpeech(step *content.Step) bool {
	return step.Kind == content.Speaking || step.Kind == content.Conversation
}

func (m Model) submit(answer string) (tea.Model, tea.Cmd) {
	_, _, step, ok := m.current()
	if !ok {
		m.screen = screenDone
		return m, nil
	}

	result, err := m.runner.Submit(context.Background(), step, answer)
	if err != nil {
		m.err = err.Error()
		// Persistence problems must not silently drop the learner's answer, so the
		// screen stays put and shows the error.
		return m, nil
	}

	m.feedback = result.Outcome.Explanation
	m.correct = result.Outcome.Correct
	if result.Ungraded {
		m.feedback = result.Outcome.Explanation
	}
	m.screen = screenFeedback
	return m, nil
}

func (m Model) transcribe(wav []byte) tea.Cmd {
	client := m.runner.Client
	language := "ja"
	return func() tea.Msg {
		if hasSpeech, _, err := speech.HasSpeech(wav, speech.SilenceFloor); err == nil && !hasSpeech {
			return speech.Transcription{Text: "", Ambiguous: true}
		}
		if client == nil {
			return speech.Transcription{}
		}
		got, err := client.Transcribe(context.Background(), wav, language)
		if err != nil {
			return speech.Transcription{}
		}
		return got
	}
}

func (m *Model) advance() {
	m.screen = screenPrompt
	m.feedback = ""
	m.transcript = ""
	m.pendingWav = nil
	m.input.SetValue("")

	unit, lesson, step, ok := m.current()
	if !ok {
		m.screen = screenDone
		return
	}
	_ = step

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

	b.WriteString(titleStyle.Render(stepLabel(step)) + "\n")
	b.WriteString(step.Prompt + "\n\n")

	if step.Target != nil {
		b.WriteString(japaneseStyle.Render(surface(*step.Target)) + "\n")
		b.WriteString(dimStyle.Render(step.Target.Romaji) + "\n\n")
	}

	if needsSpeech(step) {
		switch {
		case m.recording:
			b.WriteString(badStyle.Render("● recording") + dimStyle.Render(" — press space to stop") + "\n")
		case m.err != "":
			b.WriteString(badStyle.Render(m.err) + "\n")
			b.WriteString(dimStyle.Render("r to record, esc to skip") + "\n")
		default:
			b.WriteString(dimStyle.Render("r to record, then space to stop · esc to skip") + "\n")
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

func stepLabel(step *content.Step) string {
	return strings.ToUpper(string(step.Kind))
}

func surface(t content.Text) string {
	if t.Kanji != "" {
		return t.Kanji
	}
	return t.Kana
}

// Run starts the session UI.
func Run(runner *session.Runner, units []*content.Unit) error {
	_, err := tea.NewProgram(New(runner, units), tea.WithAltScreen()).Run()
	return err
}
