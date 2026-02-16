package ui

import (
	"codeberg.org/clambin/bubbles/stream"
	tea "github.com/charmbracelet/bubbletea"
)

var _ tea.Model = logViewer{}

type logViewer struct {
	tea.Model
}

// Update is needed here: otherwise the call in UI.Update() calls Stream's Update() method directly,
// which returns a Stream instead of a logViewer and store that in UI.panes.  UI.resize() will then panic.
func (l logViewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	l.Model, cmd = l.Model.Update(msg)
	return l, cmd
}

func (l logViewer) Size(width, height int) logViewer {
	l.Model = l.Model.(stream.Stream).Size(width, height)
	return l
}
