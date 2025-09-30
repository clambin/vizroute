package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"codeberg.org/clambin/bubbles/colors"
	"codeberg.org/clambin/bubbles/frame"
	"codeberg.org/clambin/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/clambin/pinger/pkg/ping"
	"github.com/clambin/pinger/pkg/ping/icmp"
	"github.com/clambin/vizroute/internal/discover"
	"github.com/clambin/vizroute/internal/tui"
)

var (
	ipv6    = flag.Bool("6", false, "Use IPv6")
	debug   = flag.Bool("debug", false, "Enable debug logging")
	maxHops = flag.Int("maxhops", 20, "Maximum number of hops to try")

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

	var path discover.Path
	ui := tui.NewController(target, &path, styles)
	var handlerOptions slog.HandlerOptions
	if *debug {
		handlerOptions.Level = slog.LevelDebug
	}
	l := slog.New(slog.NewTextHandler(ui.LogWriter(), &handlerOptions))

	var tp = icmp.IPv4
	if *ipv6 {
		tp = icmp.IPv6
	}

	s, err := icmp.New(tp, l.With("socket", tp))
	if err != nil {
		l.Error("failed to create icmp listener", "err", err)
		os.Exit(1)
	}
	go s.Serve(ctx)

	addr, err := s.Resolve(target)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error resolving host %q: %s\n", flag.Arg(0), err)
		os.Exit(1)
	}

	go func() {
		if err = discover.Discover(ctx, &path, addr, s, uint8(*maxHops), l); err == nil {
			ping.Ping(ctx, path.Hops, s, time.Second, 5*time.Second, l)
		}
	}()

	a := tea.NewProgram(ui, tea.WithAltScreen(), tea.WithoutCatchPanics())
	if _, err = a.Run(); err != nil {
		panic(err)
	}
}
