package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/clambin/vizroute/internal/icmp"
	"github.com/clambin/vizroute/internal/ping"
	"github.com/clambin/vizroute/internal/ui"
	"github.com/rivo/tview"
	"log/slog"
	"os"
	"os/signal"
	"time"
	//_ "net/http/pprof"
)

var (
	ipv6 = flag.Bool("6", false, "Use IPv6")
)

var a *tview.Application

func main() {
	flag.Parse()
	/*
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	*/
	var tp = icmp.IPv4
	if *ipv6 {
		tp = icmp.IPv6
	}
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	s := icmp.New(tp, l)
	s.Timeout = time.Second

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if flag.NArg() != 1 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: traceroute <host>\n")
		os.Exit(1)
	}

	addr, err := s.Resolve(flag.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error resolving host %q: %s\n", flag.Arg(0), err)
		os.Exit(1)
	}

	var p ping.Path
	go func() {
		if err = p.Run(ctx, s, addr, l); err != nil {
			panic(err)
		}
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
