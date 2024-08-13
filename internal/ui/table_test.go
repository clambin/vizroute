package ui

import (
	"github.com/clambin/vizroute/internal/ping"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
)

func TestRefreshingTable_Refresh(t *testing.T) {
	packets := []struct {
		hop     uint8
		ip      string
		up      bool
		latency time.Duration
	}{
		{1, "192.168.0.1", false, 0},
		{1, "192.168.0.1", true, 10 * time.Millisecond},
		{3, "192.168.0.2", false, 0},
		{3, "192.168.0.2", true, 20 * time.Millisecond},
	}

	var path ping.Path
	for _, packet := range packets {
		h := path.Add(packet.hop, net.ParseIP(packet.ip))
		h.Sent()
		h.Received(packet.up, packet.latency)
	}

	table := RefreshingTable{Path: &path, Table: tview.NewTable()}
	table.Refresh()

	rows := 1 + len(path.Hops())
	var cols = table.GetColumnCount()
	content := make([][]string, rows)
	for r := range rows {
		content[r] = make([]string, cols)
		for c := range cols {
			content[r][c] = table.GetCell(r, c).Text
		}
	}
	want := [][]string{
		{"hop", "addr", "name", "sent", "rcvd", "latency", "", "loss", ""},
		{"1", "192.168.0.1", "", "1", "1", "10.0ms", "|*****-----|", "0.0%", "|----------|"},
		{"2", "", "", "", "", "", "", "", ""},
		{"3", "192.168.0.2", "", "1", "1", "20.0ms", "|**********|", "0.0%", "|----------|"},
	}
	require.Equal(t, len(want), len(content))
	for i, row := range content {
		assert.Equal(t, want[i], row)
	}
}
