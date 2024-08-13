package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/clambin/vizroute/internal/icmp"
	"github.com/clambin/vizroute/internal/ping"
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
)

var a *tview.Application

func main() {
	flag.Parse()
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
	tui := ui.New(&p, *showLogs)

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	var output io.Writer = os.Stderr
	if *showLogs {
		output = tui.LogViewer()
	}
	l := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: level}))

	s := icmp.New(tp, l.With("socket", tp))
	//s.Timeout = time.Second

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
	a = tview.NewApplication().SetRoot(tui.Root, true)
	go tui.Update(ctx, a, time.Second)
	_ = a.Run()
}
