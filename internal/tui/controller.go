package tui

import (
	"io"
	"time"

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

type paneId int

const (
	viewPath paneId = iota
	viewLogs
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ tea.Model = Controller{}

// Controller is the main controller for the TUI
type Controller struct {
	helpViewer help.Model
	pathViewer tea.Model
	logViewer  tea.Model
	keyMap     KeyMap
	activePane paneId
}

var _ Tracer = (*tracer.Tracer)(nil)

type Tracer interface {
	Hops() []*tracer.HopStats
	ResetStats()
}

func NewController(target string, trace Tracer, styles table.Styles) Controller {
	return Controller{
		keyMap: DefaultKeyMap(),
		pathViewer: &pathViewer{
			target:          target,
			table:           table.NewTable("route to "+target, columns, nil, styles, table.DefaultKeyMap()),
			tracer:          trace,
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

func (c Controller) WithTracer(trace Tracer) Controller {
	c.pathViewer.(*pathViewer).tracer = trace
	return c
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
		case key.Matches(msg, c.keyMap.ResetStats):
			c.pathViewer.(pathViewer).tracer.ResetStats()
		}
	}
	return c, tea.Batch(cmds...)
}

func (c Controller) View() string {
	var body, footer string
	switch c.activePane {
	case viewPath:
		body = c.pathViewer.View()
		bindings := c.keyMap.ShortHelp()
		bindings = append(bindings, c.keyMap.TableKeys.ShortHelp()...)
		footer = c.helpViewer.ShortHelpView(bindings)
	case viewLogs:
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
