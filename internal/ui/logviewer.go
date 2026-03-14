package ui

import (
	tea "charm.land/bubbletea/v2"
	"codeberg.org/clambin/bubbles/stream"
)

type logViewer struct {
	stream.Stream
}

func (l logViewer) Update(msg tea.Msg) (logViewer, tea.Cmd) {
	var cmd tea.Cmd
	l.Stream, cmd = l.Stream.Update(msg)
	return l, cmd
}

func (l logViewer) Size(width, height int) logViewer {
	l.Stream = l.Stream.Size(width, height)
	return l
}
