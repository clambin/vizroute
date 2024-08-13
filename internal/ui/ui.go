package ui

import (
	"github.com/clambin/vizroute/internal/ping"
	"github.com/rivo/tview"
)

type UI struct {
	Table     RefreshingTable
	LogViewer *tview.TextView
	Grid      *tview.Grid
}

func New(path *ping.Path) *UI {
	ui := UI{
		Table: RefreshingTable{
			Table: tview.NewTable(),
			Path:  path,
		},
		LogViewer: tview.NewTextView(),
		Grid:      tview.NewGrid(),
	}
	ui.LogViewer.SetScrollable(true)
	ui.LogViewer.ScrollToEnd()
	ui.Grid.AddItem(ui.Table, 0, 0, 1, 1, 0, 0, true)
	ui.Grid.AddItem(ui.LogViewer, 1, 0, 1, 1, 0, 0, false)
	return &ui
}
