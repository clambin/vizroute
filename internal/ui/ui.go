package ui

import (
	"context"
	"github.com/clambin/vizroute/internal/discover"
	"github.com/rivo/tview"
	"time"
)

type UI struct {
	Root      *tview.Grid
	LogViewer *tview.TextView
	table     RefreshingTable
}

type Application interface {
	QueueUpdateDraw(func()) *tview.Application
}

func New(path *discover.Path, viewLogs bool) *UI {
	ui := UI{
		table: RefreshingTable{
			Table: tview.NewTable(),
			Path:  path,
		},
		Root: tview.NewGrid(),
	}
	ui.Root.AddItem(ui.table, 0, 0, 1, 1, 0, 0, true)
	if viewLogs {
		ui.LogViewer = tview.NewTextView()
		ui.LogViewer.SetBorder(true).SetTitle("logs").SetTitleAlign(tview.AlignLeft)
		ui.LogViewer.SetScrollable(true).ScrollToEnd()
		ui.Root.AddItem(ui.LogViewer, 1, 0, 1, 1, 0, 0, false)
	}
	return &ui
}

func (u *UI) Update(ctx context.Context, app Application, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			app.QueueUpdateDraw(func() {
				u.table.Refresh()
			})
		}
	}
}
