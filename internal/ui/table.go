package ui

import (
	"github.com/clambin/vizroute/internal/ping"
	"github.com/rivo/tview"
	"net"
	"strconv"
)

type RefreshingTable struct {
	*tview.Table
	*ping.Path
}

func (t *RefreshingTable) Refresh() {
	t.Clear()
	maxLatency := t.Path.MaxLatency()

	for i, col := range []string{"hop", "addr", "name", "sent", "rcvd", "latency", "", "loss", ""} {
		t.SetCell(0, i, tview.NewTableCell(col))
	}
	for r, hop := range t.Path.Hops() {
		var col int
		t.SetCell(r+1, 0, tview.NewTableCell(strconv.Itoa(1+r)))
		if hop == nil {
			continue
		}
		col++
		addr := hop.Addr().String()
		t.SetCell(r+1, col, tview.NewTableCell(addr))
		col++
		ipAddresses, err := net.LookupAddr(addr)
		if err != nil {
			ipAddresses = []string{""}
		}
		t.SetCell(r+1, col, tview.NewTableCell(ipAddresses[0]))
		col++
		var packets string
		n := hop.Packets()
		if n > 0 {
			packets = strconv.Itoa(n)
		}
		t.SetCell(r+1, col, tview.NewTableCell(packets).SetAlign(tview.AlignRight))
		col++
		packets = ""
		if rcvd := int(float64(n) * hop.Availability()); rcvd > 0 {
			packets = strconv.Itoa(rcvd)
		}
		t.SetCell(r+1, col, tview.NewTableCell(packets).SetAlign(tview.AlignRight))
		col++
		latency := hop.Latency()
		if latency > 0 {
			t.SetCell(r+1, col, tview.NewTableCell(strconv.FormatFloat(1000*latency.Seconds(), 'f', 1, 64)+"ms").SetAlign(tview.AlignRight))
			col++
			t.SetCell(r+1, col, tview.NewTableCell(Gradient(latency.Seconds(), maxLatency.Seconds(), 12)))
			col++
			loss := 1 - hop.Availability()
			t.SetCell(r+1, col, tview.NewTableCell(strconv.FormatFloat(100*loss, 'f', 1, 64)+"%").SetAlign(tview.AlignRight))
			col++
			t.SetCell(r+1, col, tview.NewTableCell(Gradient(loss, 1, 12)))
			col++
		}
	}
}
