package ui

import (
	"context"
	"github.com/clambin/vizroute/internal/ping"
	"github.com/rivo/tview"
	"io"
	"time"
)

type UI struct {
	Root      *tview.Grid
	logViewer *tview.TextView
	table     RefreshingTable
}

func New(path *ping.Path, viewLogs bool) *UI {
	ui := UI{
		table: RefreshingTable{
			Table: tview.NewTable(),
			Path:  path,
		},
		Root: tview.NewGrid(),
	}
	ui.Root.AddItem(ui.table, 0, 0, 1, 1, 0, 0, true)
	if viewLogs {
		ui.logViewer = tview.NewTextView()
		ui.logViewer.SetBorder(true).SetTitle("logs").SetTitleAlign(tview.AlignLeft)
		ui.logViewer.SetScrollable(true).ScrollToEnd()
		ui.Root.AddItem(ui.logViewer, 1, 0, 1, 1, 0, 0, false)
	}
	return &ui
}

func (u *UI) Update(ctx context.Context, app *tview.Application, interval time.Duration) {
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

func (u *UI) LogViewer() io.Writer {
	return u.logViewer
}
