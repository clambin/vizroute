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

func NewRefreshingTable(target string, path *discover.Path) *RefreshingTable {
	table := RefreshingTable{
		Table: tview.NewTable(),
		Path:  path,
	}
	table.Table.SetEvaluateAllRows(true).
		SetFixed(1, 0).
		SetSelectable(true, false).
		Select(1, 0).
		SetBorder(true).
		SetBorderPadding(0, 0, 1, 1)
	table.Table.SetTitle(" traceroute: " + target + " ")
	table.populateTable()
	return &table
}

func (t *RefreshingTable) populateTable() {
	columns := []string{"hop", "addr", "name", "sent", "rcvd", "latency", "", "loss", ""}
	for i, col := range columns {
		t.SetCell(0, i, tview.NewTableCell(col).SetSelectable(false))
	}
	for i, hop := range t.Path.Hops {
		t.Table.SetCell(1+i, 0, tview.NewTableCell(strconv.Itoa(i+1)).SetAlign(tview.AlignRight)) // hop
		if hop == nil {
			continue
		}
		t.Table.SetCell(i+1, 1, tview.NewTableCell(hop.IP.String())) // addr
		ipAddresses, err := net.LookupAddr(hop.IP.String())
		if err != nil {
			ipAddresses = []string{""}
		}
		t.Table.SetCell(i+1, 2, tview.NewTableCell(ipAddresses[0]))                // name
		t.Table.SetCell(i+1, 3, tview.NewTableCell("").SetAlign(tview.AlignRight)) // sent
		t.Table.SetCell(i+1, 4, tview.NewTableCell("").SetAlign(tview.AlignRight)) // rcvd
		t.Table.SetCell(i+1, 5, tview.NewTableCell("").SetAlign(tview.AlignRight)) // latency
		t.Table.SetCell(i+1, 6, tview.NewTableCell(""))                            // latency gradient
		t.Table.SetCell(i+1, 7, tview.NewTableCell("").SetAlign(tview.AlignRight)) // loss
		t.Table.SetCell(i+1, 8, tview.NewTableCell(""))                            // loss gradient
	}
}

func (t *RefreshingTable) Refresh() {
	if len(t.Path.Hops)+1 > t.Table.GetRowCount() {
		t.populateTable()
	}
	stats := getHopStatistics(t.Path)
	maxLatency := getMaxLatency(stats)

	for r, hop := range stats {
		if hop == nil {
			continue
		}
		if hop.Sent > 0 && hop.Received > 0 {
			t.Table.GetCell(r+1, 3).Text = strconv.Itoa(hop.Received)
		}
		if hop.Received > 0 {
			t.Table.GetCell(r+1, 4).Text = strconv.Itoa(hop.Received)
		}
		if hop.Latency > 0 {
			t.Table.GetCell(r+1, 5).Text = strconv.FormatFloat(1000*hop.Latency.Seconds(), 'f', 1, 64) + "ms"
			t.Table.GetCell(r+1, 6).Text = Gradient(hop.Latency.Seconds(), maxLatency.Seconds(), 12)
			loss := 1 - float64(hop.Received)/float64(hop.Sent)
			t.Table.GetCell(r+1, 7).Text = strconv.FormatFloat(100*loss, 'f', 1, 64) + "%"
			t.Table.GetCell(r+1, 8).Text = Gradient(loss, 1, 12)
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
