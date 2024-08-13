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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if flag.NArg() != 1 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: traceroute <host>\n")
		os.Exit(1)
	}

	var p ping.Path
	root := ui.New(&p)

	l := slog.New(slog.NewTextHandler(root.LogViewer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	s := icmp.New(tp, l)
	s.Timeout = time.Second

	addr, err := s.Resolve(flag.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error resolving host %q: %s\n", flag.Arg(0), err)
		os.Exit(1)
	}

	go func() {
		if err = p.Run(ctx, s, addr, l); err != nil {
			panic(err)
		}
	}()
	a = tview.NewApplication().SetRoot(root.Grid, true)
	a.SetFocus(root.LogViewer)
	go update(ctx, a, &root.Table, time.Second)
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
