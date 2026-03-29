package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TickMsg is emitted every refresh cycle.
type TickMsg struct {
	Time time.Time
}

// Init is the Bubble Tea initialization hook.
func (m *TUIModel) Init() tea.Cmd {
	return getTick()
}

// Update is the Bubble Tea update function.
func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case TickMsg:
		m.Tick++
		m.RunningTime = time.Since(m.StartTime)
		return m, getTick()
	}

	return m, nil
}

// View is the Bubble Tea view function.
func (m *TUIModel) View() string {
	return m.Render()
}

func getTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}
