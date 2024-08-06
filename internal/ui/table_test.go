package ui

import (
	"github.com/clambin/vizroute/internal/icmp"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"time"
)

func TestRefreshingTable_Refresh(t *testing.T) {
	var path icmp.Path
	path.Add(1, net.ParseIP("192.168.0.1")).Measurement(true, time.Second)
	//path.Add(2, nil)
	path.Add(3, net.ParseIP("192.168.0.2")).Measurement(true, 2*time.Second)

	table := RefreshingTable{Path: &path, Table: tview.NewTable()}
	table.Refresh()

	rows := 1 + len(path.Hops())
	const cols = 7
	content := make([][]string, rows)
	for r := range rows {
		content[r] = make([]string, cols)
		for c := range cols {
			content[r][c] = table.GetCell(r, c).Text
		}
	}
	assert.Equal(t, [][]string{
		{"hop", "addr", "name", "latency", "", "loss", ""},
		{"1", "192.168.0.1", "", "1s", "|*****-----|", "0.00%", "|----------|"},
		{"2", "", "", "", "", "", ""},
		{"3", "192.168.0.2", "", "2s", "|**********|", "0.00%", "|----------|"},
	}, content)
}
