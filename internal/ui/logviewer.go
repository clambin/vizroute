package ui

import (
	"codeberg.org/clambin/bubbles/frame"
	"codeberg.org/clambin/bubbles/stream"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type logViewer struct {
	stream *stream.Stream
	styles frame.Styles
}

func (l logViewer) Init() tea.Cmd {
	return l.stream.Init()
}

func (l logViewer) Update(msg tea.Msg) (logViewer, tea.Cmd) {
	cmd := l.stream.Update(msg)
	return l, cmd
}

func (l logViewer) View() string {
	return frame.Draw("logs", lipgloss.Center, l.stream.View(), l.styles)
}
