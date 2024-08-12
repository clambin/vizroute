package ui

import (
	"github.com/clambin/vizroute/internal/ping"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"time"
)

func TestRefreshingTable_Refresh(t *testing.T) {
	var path ping.Path
	path.Add(1, net.ParseIP("192.168.0.1")).Measure(false, 0)
	path.Add(1, net.ParseIP("192.168.0.1")).Measure(true, 10*time.Millisecond)
	//path.Add(2, nil)
	path.Add(3, net.ParseIP("192.168.0.2")).Measure(false, 0)
	path.Add(3, net.ParseIP("192.168.0.2")).Measure(true, 20*time.Millisecond)

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
	assert.Equal(t, [][]string{
		{"hop", "addr", "name", "packets", "latency", "", "loss", ""},
		{"1", "192.168.0.1", "", "1", "10.0ms", "|*****-----|", "0.0%", "|----------|"},
		{"2", "", "", "", "", "", "", ""},
		{"3", "192.168.0.2", "", "1", "20.0ms", "|**********|", "0.0%", "|----------|"},
	}, content)
}
