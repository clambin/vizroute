package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"codeberg.org/clambin/bubbles/colors"
	"codeberg.org/clambin/bubbles/frame"
	"codeberg.org/clambin/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/clambin/vizroute/internal/tracer"
	"github.com/clambin/vizroute/internal/ui"
	"github.com/clambin/vizroute/ping"
)

var (
	ipv6    = flag.Bool("6", false, "Use IPv6")
	debug   = flag.Bool("debug", false, "Enable debug logging")
	maxHops = flag.Int("maxhops", 10, "Maximum number of hops to try")

	styles = table.Styles{
		Frame: frame.Styles{
			Title:  lipgloss.NewStyle().Foreground(colors.Green),
			Border: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colors.Blue),
		},
		Header: lipgloss.NewStyle().Foreground(colors.Blue),
		//Selected: lipgloss.NewStyle().Foreground(colors.Black).Background(colors.Blue),
		//Cell:     lipgloss.NewStyle(),
	}
)

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if flag.NArg() != 1 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: traceroute <host>\n")
		os.Exit(1)
	}
	target := flag.Arg(0)

	tui := ui.New(target, nil, styles)
	var handlerOptions slog.HandlerOptions
	if *debug {
		handlerOptions.Level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(tui.LogWriter(), &handlerOptions))

	opts := []ping.SocketOption{ping.WithIPv4(), ping.WithLogger(logger.With("component", "socket"))}
	if *ipv6 {
		opts[0] = ping.WithIPv6()
	}

	s, err := ping.New(opts...)
	if err != nil {
		logger.Error("failed to create icmp listener", "err", err)
		os.Exit(1)
	}
	go s.Serve(ctx)

	if _, err = s.Resolve(target); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error resolving host %q: %s\n", flag.Arg(0), err)
		os.Exit(1)
	}

	tr := tracer.NewTracer(s, logger.With("component", "tracer"))
	tui = tui.WithTracer(tr)

	go func() {
		if err := tr.Run(ctx, target, *maxHops); err != nil {
			logger.Error("tracer failed", "err", err)
			panic(err)
		}
	}()

	a := tea.NewProgram(tui, tea.WithAltScreen(), tea.WithoutCatchPanics())
	if _, err = a.Run(); err != nil {
		panic(err)
	}
	cancel()
}
