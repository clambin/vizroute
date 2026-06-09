package ui

import (
	tea "charm.land/bubbletea/v2"
	"codeberg.org/clambin/bubbles/stream"
)

type logViewer struct {
	stream.Model
}

func (l logViewer) Update(msg tea.Msg) (logViewer, tea.Cmd) {
	var cmd tea.Cmd
	l.Model, cmd = l.Model.Update(msg)
	return l, cmd
}

func (l logViewer) Size(width, height int) logViewer {
	l.Model = l.Model.SetSize(width, height)
	return l
}
