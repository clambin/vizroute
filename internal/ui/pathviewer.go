package ui

import (
	"fmt"
	"time"

	"codeberg.org/clambin/bubbles/table"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clambin/vizroute/internal/tracer"
)

var _ tea.Model = pathViewer{}

// pathViewer is a table viewer for the path
type pathViewer struct {
	target          string
	table           tea.Model
	tracer          Tracer
	latencyProgress progress.Model
	lossProgress    progress.Model
}

func (p pathViewer) Init() tea.Cmd {
	return refreshPathCmd(refreshInterval)
}

func (p pathViewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case refreshPathMsg:
		return p, tea.Batch(
			p.updateTableCmd(),
			refreshPathCmd(refreshInterval),
		)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return p, tea.Quit
		}
	}
	p.table, cmd = p.table.Update(msg)
	return p, cmd
}

func (p pathViewer) updateTableCmd() tea.Cmd {
	return func() tea.Msg {
		return table.SetRowsMsg{Rows: p.hopsToRows()}
	}
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

func (p pathViewer) View() string {
	return p.table.View()
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
