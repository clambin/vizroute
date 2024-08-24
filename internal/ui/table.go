package ui

import (
	"github.com/clambin/vizroute/internal/discover"
	"github.com/rivo/tview"
	"net"
	"strconv"
	"time"
)

type RefreshingTable struct {
	*tview.Table
	*discover.Path
}

func (t *RefreshingTable) Refresh() {
	t.Clear()

	stats := getHopStatistics(t.Path)
	maxLatency := getMaxLatency(stats)

	for i, col := range []string{"hop", "addr", "name", "sent", "rcvd", "latency", "", "loss", ""} {
		t.SetCell(0, i, tview.NewTableCell(col))
	}
	for r, hop := range stats {
		var col int
		t.SetCell(r+1, 0, tview.NewTableCell(strconv.Itoa(r+1)))
		if hop == nil {
			continue
		}
		col++
		addr := hop.addr.String()
		t.SetCell(r+1, col, tview.NewTableCell(addr))
		col++
		ipAddresses, err := net.LookupAddr(addr)
		if err != nil {
			ipAddresses = []string{""}
		}
		t.SetCell(r+1, col, tview.NewTableCell(ipAddresses[0]))
		col++
		var packets string
		if hop.sent > 0 {
			packets = strconv.Itoa(hop.sent)
		}
		t.SetCell(r+1, col, tview.NewTableCell(packets).SetAlign(tview.AlignRight))
		col++
		packets = ""
		if hop.rcvd > 0 {
			packets = strconv.Itoa(hop.rcvd)
		}
		t.SetCell(r+1, col, tview.NewTableCell(packets).SetAlign(tview.AlignRight))
		col++
		latency := hop.latency
		if latency > 0 {
			t.SetCell(r+1, col, tview.NewTableCell(strconv.FormatFloat(1000*latency.Seconds(), 'f', 1, 64)+"ms").SetAlign(tview.AlignRight))
			col++
			t.SetCell(r+1, col, tview.NewTableCell(Gradient(latency.Seconds(), maxLatency.Seconds(), 12)))
			col++
			loss := 1 - float64(hop.rcvd)/float64(hop.sent)
			t.SetCell(r+1, col, tview.NewTableCell(strconv.FormatFloat(100*loss, 'f', 1, 64)+"%").SetAlign(tview.AlignRight))
			col++
			t.SetCell(r+1, col, tview.NewTableCell(Gradient(loss, 1, 12)))
			col++
		}
	}
}

type hopStatistics struct {
	addr    net.IP
	sent    int
	rcvd    int
	latency time.Duration
}

func getHopStatistics(path *discover.Path) []*hopStatistics {
	statistics := make([]*hopStatistics, path.Len())
	for i, hop := range path.Hops {
		if hop != nil {
			sent, received, latency := hop.Statistics()
			statistics[i] = &hopStatistics{
				addr:    hop.IP,
				sent:    sent,
				rcvd:    received,
				latency: latency,
			}
		}
	}
	return statistics
}

func getMaxLatency(hops []*hopStatistics) time.Duration {
	var maxLatency time.Duration
	for _, hop := range hops {
		if hop != nil && hop.latency > maxLatency {
			maxLatency = hop.latency
		}
	}
	return maxLatency
}
