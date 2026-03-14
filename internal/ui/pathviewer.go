package ui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"codeberg.org/clambin/bubbles/table"
	"github.com/clambin/vizroute/internal/tracer"
)

// pathViewer is a table viewer for the path
type pathViewer struct {
	latencyProgress progress.Model
	lossProgress    progress.Model
	tracer          Tracer
	table.Table
}

func (p pathViewer) Init() tea.Cmd {
	return tea.Batch(
		p.Table.Init(),
		refreshPathCmd(refreshInterval),
	)
}

func (p pathViewer) Update(msg tea.Msg) (pathViewer, tea.Cmd) {
	switch msg := msg.(type) {
	case refreshPathMsg:
		p.Table = p.Rows(p.hopsToRows())
		return p, refreshPathCmd(refreshInterval)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return p, tea.Quit
		}
	}
	var cmd tea.Cmd
	p.Table, cmd = p.Table.Update(msg)
	return p, cmd
}

func (p pathViewer) Size(width, height int) pathViewer {
	p.Table = p.Table.Size(width, height)
	return p
}

func (p pathViewer) hopsToRows() []table.Row {
	hops := p.tracer.Hops()
	rows := make([]table.Row, len(hops))
	maxLatency := maxLatency(hops)
	for i, hop := range hops {
		if hop == nil {
			rows[i] = table.Row{i + 1}
			continue
		}
		rows[i] = p.formatRow(hop, i+1, maxLatency)
	}
	return rows
}

func maxLatency(hops []*tracer.HopStats) time.Duration {
	var maxLatency time.Duration
	for _, hop := range hops {
		if hop == nil {
			continue
		}
		maxLatency = max(maxLatency, hop.MedianRTT())
	}
	return maxLatency
}

func (p pathViewer) formatRow(hop *tracer.HopStats, c int, maxLatency time.Duration) table.Row {
	var latency string
	if lat := hop.MedianRTT(); lat > 0 {
		latency = p.latencyProgress.ViewAs(lat.Seconds()/maxLatency.Seconds()) +
			" " +
			fmt.Sprintf("%6.1fms", lat.Seconds()*1000)
	}
	//var packetLoss string
	//if statistics.Received > 0 {
	packetLoss := p.lossProgress.ViewAs(hop.Loss())
	//}
	sent, received := hop.PacketCount()
	return table.Row{
		c,
		hop.IP().String(),
		hop.Addr(),
		sent,
		received,
		latency,
		packetLoss,
	}
}
