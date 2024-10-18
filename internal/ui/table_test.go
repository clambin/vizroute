package ui

import (
	"fmt"
	"github.com/clambin/pinger/pkg/ping"
	"github.com/clambin/pinger/pkg/ping/icmp"
	"github.com/clambin/vizroute/internal/discover"
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

	var path discover.Path
	for range 3 {
		path.AddHop()
	}
	for idx, packet := range packets {
		h := ping.Target{IP: net.ParseIP(packet.ip)}
		h.Sent(icmp.SequenceNumber(idx + 1))
		time.Sleep(packet.latency / 2)
		h.Received(packet.up, icmp.SequenceNumber(idx+1))
		path.SetHop(int(packet.hop-1), &h)
	}

	table := NewRefreshingTable("", &path)
	table.Refresh()

	rows := 1 + path.Len()
	cols := table.GetColumnCount()
	content := make([][]string, rows)
	for r := range rows {
		content[r] = make([]string, cols)
		for c := range cols {
			content[r][c] = table.GetCell(r, c).Text
		}
	}
	const ignoreCell = "<ignore>"
	want := [][]string{
		{"hop", "addr", "name", "sent", "rcvd", "latency", "", "loss", ""},
		{"1", "192.168.0.1", "", "1", "1", ignoreCell, ignoreCell, "0.0%", "|----------|"},
		{"2", "", "", "", "", "", "", "", ""},
		{"3", "192.168.0.2", "", "1", "1", ignoreCell, ignoreCell, "0.0%", "|----------|"},
	}
	require.Equal(t, len(want), len(content))
	for r, row := range content {
		require.Equal(t, len(want[r]), len(row))
		for c, cell := range row {
			if want[r][c] != ignoreCell {
				assert.Equalf(t, want[r][c], cell, fmt.Sprintf("(row: %d, col: %d)", r, c))
			}
		}
	}
}

func readTable(table *RefreshingTable) [][]string {
	rows := table.GetRowCount()
	content := make([][]string, rows)
	cols := table.GetColumnCount()
	for r := range rows {
		content[r] = make([]string, cols)
		for c := range cols {
			content[r][c] = table.GetCell(r, c).Text
		}
	}
	return content
}
