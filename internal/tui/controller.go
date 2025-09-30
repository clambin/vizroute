package tui

import (
	"fmt"
	"io"
	"net"
	"time"

	"codeberg.org/clambin/bubbles/frame"
	"codeberg.org/clambin/bubbles/stream"
	"codeberg.org/clambin/bubbles/table"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/clambin/pinger/pkg/ping"
	"github.com/clambin/vizroute/internal/discover"
)

const refreshInterval = 250 * time.Millisecond

var (
	columns = []table.Column{
		{Name: "Hop", Width: 5, RowStyle: table.CellStyle{Style: lipgloss.NewStyle().Align(lipgloss.Right)}},
		{Name: "Addr", Width: 38},
		{Name: "Name"},
		{Name: "Sent", Width: 8, RowStyle: table.CellStyle{Style: lipgloss.NewStyle().Align(lipgloss.Right)}},
		{Name: "Rcvd", Width: 8, RowStyle: table.CellStyle{Style: lipgloss.NewStyle().Align(lipgloss.Right)}},
		{Name: "Latency", Width: 30},
		{Name: "Loss", Width: 30},
	}
)

type refreshPathMsg struct{}

func refreshPathCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return refreshPathMsg{}
	})
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ tea.Model = Controller{}

// Controller is the main controller for the TUI
type Controller struct {
	keyMap     KeyMap
	pathViewer tea.Model
	logViewer  tea.Model
	helpViewer help.Model
	activePane int
}

func NewController(target string, path *discover.Path, styles table.Styles) Controller {
	return Controller{
		keyMap: DefaultKeyMap(),
		pathViewer: pathViewer{
			target:          target,
			table:           table.NewTable("route to "+target, columns, nil, styles, table.DefaultKeyMap()),
			path:            path,
			latencyProgress: progress.New(progress.WithWidth(columns[5].Width-10), progress.WithoutPercentage()),
			lossProgress:    progress.New(progress.WithWidth(columns[6].Width - 1)),
		},
		logViewer: logViewer{
			model:  stream.NewStream(80, 25, stream.WithShowToggles(true)),
			styles: styles.Frame,
		},
		helpViewer: help.New(),
	}
}

func (c Controller) LogWriter() io.Writer {
	return c.logViewer.(logViewer).model.(io.Writer)
}

func (c Controller) Init() tea.Cmd {
	return tea.Batch(
		c.pathViewer.Init(),
		c.logViewer.Init(),
	)
}

func (c Controller) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	c.pathViewer, cmd = c.pathViewer.Update(msg)
	cmds = append(cmds, cmd)
	c.logViewer, cmd = c.logViewer.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.helpViewer.Width = msg.Width
		helpHeight := lipgloss.Height(c.helpViewer.ShortHelpView(c.keyMap.ShortHelp()))
		c.pathViewer, cmd = c.pathViewer.Update(table.SetSizeMsg{Width: msg.Width, Height: msg.Height - helpHeight})
		cmds = append(cmds, cmd)
		borderWidth, borderHeight := c.logViewer.(logViewer).styles.BorderSize()
		c.logViewer, cmd = c.logViewer.Update(stream.SetSizeMsg{Width: msg.Width - borderWidth, Height: msg.Height - helpHeight - borderHeight})
		cmds = append(cmds, cmd)
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, c.keyMap.Quit):
			cmds = append(cmds, tea.Quit)
		case key.Matches(msg, c.keyMap.NextPane):
			c.activePane = (c.activePane + 1) % 2
		}
	}
	return c, tea.Batch(cmds...)
}

func (c Controller) View() string {
	var body, footer string
	switch c.activePane {
	case 0:
		body = c.pathViewer.View()
		bindings := c.keyMap.ShortHelp()
		bindings = append(bindings, c.keyMap.TableKeys.ShortHelp()...)
		footer = c.helpViewer.ShortHelpView(bindings)
	case 1:
		body = c.logViewer.View()
		bindings := c.keyMap.ShortHelp()
		bindings = append(bindings, c.keyMap.TableKeys.ShortHelp()...)
		bindings = append(bindings, c.keyMap.StreamKeys.ShortHelp()...)
		footer = c.helpViewer.ShortHelpView(bindings)
	}
	return lipgloss.JoinVertical(lipgloss.Top, body, footer)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ help.KeyMap = KeyMap{}

type KeyMap struct {
	Quit       key.Binding
	NextPane   key.Binding
	TableKeys  table.KeyMap
	StreamKeys stream.KeyMap
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.NextPane}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		k.ShortHelp(),
	}
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:       key.NewBinding(key.WithKeys(tea.KeyCtrlC.String(), "q"), key.WithHelp("ctr+c/q", "quit the program")),
		NextPane:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch between the path and logs")),
		TableKeys:  table.DefaultKeyMap(),
		StreamKeys: stream.DefaultKeyMap(),
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ tea.Model = pathViewer{}

// pathViewer is a table viewer for the path
type pathViewer struct {
	target          string
	table           tea.Model
	path            *discover.Path
	latencyProgress progress.Model
	lossProgress    progress.Model
}

func (p pathViewer) Init() tea.Cmd {
	return refreshPathCmd(refreshInterval)
}

func (p pathViewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case refreshPathMsg:
		return p, tea.Batch(
			p.updateTableCmd(),
			refreshPathCmd(refreshInterval),
		)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return p, tea.Quit
		}
	}
	p.table, cmd = p.table.Update(msg)
	return p, cmd
}

func (p pathViewer) updateTableCmd() tea.Cmd {
	return func() tea.Msg {
		return table.SetRowsMsg{Rows: p.hopsToRows()}
	}
}

func (p pathViewer) hopsToRows() []table.Row {
	path := p.path // TODO: this is a race condition vs. the path being updated in the discovery package
	rows := make([]table.Row, path.Len())
	hops := path.Hops
	maxLatency := maxLatency(hops)
	for i, hop := range hops {
		if hop == nil {
			rows[i] = table.Row{i + 1}
			continue
		}
		rows[i] = p.formatRow(hop, i+1, maxLatency)
	}
	return rows
}

func maxLatency(hops []*ping.Target) time.Duration {
	var maxLatency time.Duration
	for _, hop := range hops {
		if hop == nil {
			continue
		}
		statistics := hop.Statistics()
		if statistics.Received > 0 {
			maxLatency = max(maxLatency, statistics.Latency)
		}
	}
	return maxLatency
}

func (p pathViewer) formatRow(hop *ping.Target, c int, maxLatency time.Duration) table.Row {
	ipAddresses, err := net.LookupAddr(hop.IP.String())
	if err != nil {
		ipAddresses = []string{""}
	}
	statistics := hop.Statistics()
	var latency string
	if statistics.Latency > 0 {
		latency = p.latencyProgress.ViewAs(statistics.Latency.Seconds()/maxLatency.Seconds()) +
			" " +
			fmt.Sprintf("%6.1fms", statistics.Latency.Seconds()*1000)
	}
	var packetLoss string
	//if statistics.Received > 0 {
	packetLoss = p.lossProgress.ViewAs(float64(statistics.Sent-statistics.Received) / float64(statistics.Sent))
	//}
	return table.Row{
		c,
		hop.IP.String(),
		ipAddresses[0],
		statistics.Sent,
		statistics.Received,
		latency,
		packetLoss,
	}
}

func (p pathViewer) View() string {
	return p.table.View()
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ tea.Model = logViewer{}

type logViewer struct {
	model  tea.Model
	styles frame.Styles
}

func (l logViewer) Init() tea.Cmd {
	return l.model.Init()
}

func (l logViewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	l.model, cmd = l.model.Update(msg)
	return l, cmd
}

func (l logViewer) View() string {
	return frame.Draw("logs", lipgloss.Center, l.model.View(), l.styles)
}
