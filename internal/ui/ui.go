package ui

import (
	"io"
	"time"

	"codeberg.org/clambin/bubbles/colors"
	"codeberg.org/clambin/bubbles/frame"
	"codeberg.org/clambin/bubbles/stream"
	"codeberg.org/clambin/bubbles/table"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/clambin/vizroute/internal/tracer"
)

const refreshInterval = 250 * time.Millisecond

var (
	columns = []table.Column{
		{Name: "Hop", Width: 5, CellStyle: table.CellStyle{Style: lipgloss.NewStyle().Align(lipgloss.Right)}},
		{Name: "Addr", Width: 38},
		{Name: "Name"},
		{Name: "Sent", Width: 8, CellStyle: table.CellStyle{Style: lipgloss.NewStyle().Align(lipgloss.Right)}},
		{Name: "Rcvd", Width: 8, CellStyle: table.CellStyle{Style: lipgloss.NewStyle().Align(lipgloss.Right)}},
		{Name: "Latency", Width: 30},
		{Name: "Loss", Width: 30},
	}

	frameStyle = frame.Style{
		Title:  lipgloss.NewStyle().Foreground(colors.Green).Bold(true),
		Border: lipgloss.NewStyle().BorderForeground(colors.Blue).BorderStyle(lipgloss.RoundedBorder()),
	}
)

type refreshPathMsg struct{}

func refreshPathCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return refreshPathMsg{}
	})
}

type paneId int

const (
	viewPath paneId = iota
	viewLogs
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ Tracer = (*tracer.Tracer)(nil)

type Tracer interface {
	Hops() []*tracer.HopStats
	ResetStats()
}

var _ tea.Model = UI{}

// UI is the main controller for the TUI
type UI struct {
	activePane paneId
	pathPane   tea.Model
	logsPane   tea.Model

	helpViewer help.Model
	keyMap     KeyMap
	target     string
}

func New(target string, trace Tracer, styles table.Styles) UI {
	return UI{
		pathPane: pathViewer{
			Model:           table.New().Columns(columns).Styles(styles),
			tracer:          trace,
			latencyProgress: progress.New(progress.WithWidth(columns[5].Width-10), progress.WithoutPercentage()),
			lossProgress:    progress.New(progress.WithWidth(columns[6].Width - 1)),
		},
		logsPane: logViewer{Model: stream.New(stream.WithShowToggles(true))},

		keyMap:     DefaultKeyMap(),
		helpViewer: help.New(),
		target:     target,
	}
}

func (c UI) WithTracer(trace Tracer) UI {
	var p = c.pathPane.(pathViewer)
	p.tracer = trace
	c.pathPane = p
	return c
}

func (c UI) LogWriter() io.Writer {
	return c.logsPane.(logViewer).Model.(io.Writer)
}

func (c UI) Init() tea.Cmd {
	return tea.Batch(
		c.pathPane.Init(),
		c.logsPane.Init(),
	)
}

func (c UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return c.resize(msg.Width, msg.Height), nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, c.keyMap.Quit):
			return c, tea.Quit
		case key.Matches(msg, c.keyMap.NextPane):
			c.activePane = (c.activePane + 1) % 2
			return c, nil
		case key.Matches(msg, c.keyMap.ResetStats):
			c.pathPane.(pathViewer).tracer.ResetStats()
			return c, nil
		default:
			var cmd tea.Cmd
			switch c.activePane {
			case viewPath:
				c.pathPane, cmd = c.pathPane.Update(msg)
			case viewLogs:
				c.logsPane, cmd = c.logsPane.Update(msg)
			}
			return c, cmd
		}
	default:
		var cmds []tea.Cmd
		var cmd tea.Cmd
		c.pathPane, cmd = c.pathPane.Update(msg)
		cmds = append(cmds, cmd)
		c.logsPane, cmd = c.logsPane.Update(msg)
		cmds = append(cmds, cmd)
		return c, tea.Batch(cmds...)
	}
}

func (c UI) View() string {
	var v string
	switch c.activePane {
	case viewPath:
		v = c.pathPane.View()
	case viewLogs:
		v = c.logsPane.View()
	}
	return lipgloss.JoinVertical(lipgloss.Top,
		frame.Draw(c.target, lipgloss.Center, v, frameStyle),
		c.helpViewer.ShortHelpView(c.activeBindings()),
	)
}

func (c UI) resize(width, height int) UI {
	c.helpViewer.Width = width
	helpHeight := lipgloss.Height(c.helpViewer.ShortHelpView(c.keyMap.ShortHelp()))

	width -= frameStyle.Border.GetHorizontalBorderSize()
	height -= frameStyle.Border.GetVerticalBorderSize() + helpHeight

	c.pathPane = c.pathPane.(pathViewer).Size(width, height)
	c.logsPane = c.logsPane.(logViewer).Size(width, height)
	return c
}

func (c UI) activeBindings() []key.Binding {
	bindings := c.keyMap.ShortHelp()
	switch c.activePane {
	case viewPath:
		bindings = append(bindings, c.keyMap.TableKeys.ShortHelp()...)
	case viewLogs:
		bindings = append(bindings, c.keyMap.TableKeys.ShortHelp()...)
		bindings = append(bindings, c.keyMap.StreamKeys.ShortHelp()...)
	}
	return bindings
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ help.KeyMap = KeyMap{}

type KeyMap struct {
	Quit       key.Binding
	NextPane   key.Binding
	ResetStats key.Binding
	TableKeys  table.KeyMap
	StreamKeys stream.KeyMap
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.NextPane, k.ResetStats}
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
		ResetStats: key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "reset statistics")),
		TableKeys:  table.DefaultKeyMap(),
		StreamKeys: stream.DefaultKeyMap(),
	}
}
