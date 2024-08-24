package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/clambin/pinger/pkg/ping"
	"github.com/clambin/pinger/pkg/ping/icmp"
	"github.com/clambin/vizroute/internal/discover"
	"github.com/clambin/vizroute/internal/ui"
	"github.com/rivo/tview"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"time"
	//_ "net/http/pprof"
)

var (
	ipv6     = flag.Bool("6", false, "Use IPv6")
	debug    = flag.Bool("debug", false, "Enable debug logging")
	showLogs = flag.Bool("logs", false, "Show logging")
	maxHops  = flag.Int("maxhops", 20, "Maximum number of hops to try")
)

var a *tview.Application

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if flag.NArg() != 1 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: traceroute <host>\n")
		os.Exit(1)
	}

	var p discover.Path
	tui := ui.New(&p, *showLogs)

	var output io.Writer = os.Stderr
	if *showLogs {
		output = tui.LogViewer
	}
	var handlerOptions slog.HandlerOptions
	if *debug {
		handlerOptions.Level = slog.LevelDebug
	}
	l := slog.New(slog.NewTextHandler(output, &handlerOptions))

	var tp = icmp.IPv4
	if *ipv6 {
		tp = icmp.IPv6
	}

	s := icmp.New(tp, l.With("socket", tp))
	go s.Serve(ctx)

	addr, err := s.Resolve(flag.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error resolving host %q: %s\n", flag.Arg(0), err)
		os.Exit(1)
	}

	go func() {
		if err = discover.Discover(ctx, &p, addr, s, uint8(*maxHops), l); err == nil {
			ping.Ping(ctx, p.Hops, s, time.Second, 5*time.Second, l)
		}
	}()
	a = tview.NewApplication().SetRoot(tui.Root, true)
	go tui.Update(ctx, a, time.Second)
	_ = a.Run()
}
