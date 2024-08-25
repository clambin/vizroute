package ui

import (
	"github.com/clambin/pinger/pkg/ping"
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

	columns := []string{"hop", "addr", "name", "sent", "rcvd", "latency", "", "loss", ""}
	for i, col := range columns {
		t.SetCell(0, i, tview.NewTableCell(col))
	}
	for r, hop := range stats {
		var col int
		t.SetCell(r+1, 0, tview.NewTableCell(strconv.Itoa(r+1)))
		if hop == nil {
			continue
		}
		addr := hop.addr.String()
		loss := 1 - float64(hop.Received)/float64(hop.Sent)
		for col++; col < len(columns); col++ {
			var value string
			var alignRight bool
			switch col {
			case 1:
				value = addr
			case 2:
				ipAddresses, err := net.LookupAddr(addr)
				if err != nil {
					ipAddresses = []string{""}
				}
				value = ipAddresses[0]
			case 3:
				if hop.Sent > 0 && hop.Received > 0 {
					value = strconv.Itoa(hop.Sent)
					alignRight = true
				}
			case 4:
				if hop.Received > 0 {
					value = strconv.Itoa(hop.Received)
					alignRight = true
				}
			case 5:
				if hop.Latency > 0 {
					value = strconv.FormatFloat(1000*hop.Latency.Seconds(), 'f', 1, 64) + "ms"
					alignRight = true
				}
			case 6:
				if hop.Latency > 0 {
					value = Gradient(hop.Latency.Seconds(), maxLatency.Seconds(), 12)
				}
			case 7:
				if hop.Latency > 0 {
					value = strconv.FormatFloat(100*loss, 'f', 1, 64) + "%"
					alignRight = true
				}
			case 8:
				if hop.Latency > 0 {
					value = Gradient(loss, 1, 12)
				}
			}
			cell := tview.NewTableCell(value)
			if alignRight {
				cell.SetAlign(tview.AlignRight)
			}
			t.SetCell(r+1, col, cell)
		}
	}
}

type hopStatistics struct {
	addr net.IP
	ping.Statistics
}

func getHopStatistics(path *discover.Path) []*hopStatistics {
	statistics := make([]*hopStatistics, path.Len())
	for i, hop := range path.Hops {
		if hop != nil {
			statistics[i] = &hopStatistics{
				addr:       hop.IP,
				Statistics: hop.Statistics(),
			}
		}
	}
	return statistics
}

func getMaxLatency(hops []*hopStatistics) time.Duration {
	var maxLatency time.Duration
	for _, hop := range hops {
		if hop != nil && hop.Latency > maxLatency {
			maxLatency = hop.Latency
		}
	}
	return maxLatency
}
