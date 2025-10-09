package ui

import (
	"codeberg.org/clambin/bubbles/frame"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ tea.Model = logViewer{}

type logViewer struct {
	model  tea.Model
	styles frame.Styles
}

func (l logViewer) Init() tea.Cmd {
	return l.model.Init()
}

func (l logViewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	l.model, cmd = l.model.Update(msg)
	return l, cmd
}

func (l logViewer) View() string {
	return frame.Draw("logs", lipgloss.Center, l.model.View(), l.styles)
}
