package main

import (
	"context"
	"fmt"
	"github.com/clambin/vizroute/internal/icmp"
	"github.com/clambin/vizroute/internal/ui"
	"github.com/rivo/tview"
	"log/slog"
	"os"
	"os/signal"
	"time"
)

var a *tview.Application

func main() {
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	s := icmp.New(icmp.IPv4, l)
	s.Timeout = time.Second

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if len(os.Args) == 1 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: traceroute <host>\n")
		os.Exit(1)
	}

	addr, err := s.Resolve(os.Args[1])
	if err != nil {
		panic(err)
	}
	var p icmp.Path

	go func() {
		if err = p.Discover(ctx, s, addr); err != nil {
			panic(err)
		}
		// can't do multiple pings in parallel, so wait for Discovery to end before pinging hops
		p.Ping(ctx, s, l)
	}()

	table := &ui.RefreshingTable{Table: tview.NewTable(), Path: &p}
	a = tview.NewApplication().SetRoot(table, true)
	go update(ctx, a, table, time.Second)
	_ = a.Run()
}

func update(ctx context.Context, app *tview.Application, t *ui.RefreshingTable, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			app.QueueUpdateDraw(func() {
				t.Refresh()
			})
		}
	}
}
