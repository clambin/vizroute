package ui

import (
	"github.com/clambin/vizroute/internal/icmp"
	"github.com/rivo/tview"
	"net"
	"strconv"
)

type RefreshingTable struct {
	*tview.Table
	*icmp.Path
}

func (t *RefreshingTable) Refresh() {
	t.Clear()
	maxLatency := t.Path.MaxLatency()
	for i, col := range []string{"hop", "addr", "name", "latency", "", "loss", ""} {
		t.SetCell(0, i, tview.NewTableCell(col))
	}
	for r, hop := range t.Path.Hops() {
		t.SetCell(r+1, 0, tview.NewTableCell(strconv.Itoa(1+r)))
		if hop == nil {
			continue
		}
		addr := hop.Addr().String()
		t.SetCell(r+1, 1, tview.NewTableCell(addr))
		ipAddresses, err := net.LookupAddr(addr)
		if err != nil {
			ipAddresses = []string{""}
		}
		t.SetCell(r+1, 2, tview.NewTableCell(ipAddresses[0]))
		if latency := hop.Latency(); latency != 0 {
			t.SetCell(r+1, 3, tview.NewTableCell(latency.String()))
			t.SetCell(r+1, 4, tview.NewTableCell(Gradient(latency.Seconds(), maxLatency.Seconds(), 12)))
		}
		loss := 1 - hop.Availability()
		t.SetCell(r+1, 5, tview.NewTableCell(strconv.FormatFloat(100*loss, 'f', 2, 64)+"%"))
		t.SetCell(r+1, 6, tview.NewTableCell(Gradient(loss, 1, 12)))
	}
}
