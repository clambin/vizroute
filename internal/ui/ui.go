package ui

import (
	"io"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"codeberg.org/clambin/bubbles/colors"
	"codeberg.org/clambin/bubbles/frame"
	"codeberg.org/clambin/bubbles/stream"
	"codeberg.org/clambin/bubbles/table"
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

	helpStyle = lipgloss.NewStyle().Foreground(colors.DarkOrange3).Italic(true)
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
	pathPane pathViewer

	helpViewer help.Model
	target     string
	keyMap     KeyMap
	logsPane   logViewer

	activePane paneId
}

func New(target string, trace Tracer, styles table.Styles) UI {
	helpViewer := help.New()
	helpViewer.Styles = help.Styles{
		ShortDesc: helpStyle,
		ShortKey:  helpStyle.Bold(true),
	}
	return UI{
		pathPane: pathViewer{
			Table:  table.New().Columns(columns).Styles(styles),
			tracer: trace,
			latencyProgress: progress.New(
				progress.WithWidth(columns[5].Width-10),
				progress.WithoutPercentage(),
				progress.WithDefaultBlend(),
			),
			lossProgress: progress.New(
				progress.WithWidth(columns[6].Width-1),
				progress.WithDefaultBlend(),
			),
		},
		logsPane: logViewer{
			Stream: stream.New(stream.WithShowToggles(true)),
		},
		keyMap:     DefaultKeyMap(),
		helpViewer: helpViewer,
		target:     target,
	}
}

func (c UI) WithTracer(trace Tracer) UI {
	c.pathPane.tracer = trace
	return c
}

func (c UI) LogWriter() io.Writer {
	return c.logsPane.Stream
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
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.Quit):
			return c, tea.Quit
		case key.Matches(msg, c.keyMap.NextPane):
			c.activePane = (c.activePane + 1) % 2
			return c, nil
		case key.Matches(msg, c.keyMap.ResetStats):
			c.pathPane.tracer.ResetStats()
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

func (c UI) View() tea.View {
	var content string
	switch c.activePane {
	case viewPath:
		content = c.pathPane.View()
	case viewLogs:
		content = c.logsPane.View()
	}

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Top,
		frame.Render(c.target, lipgloss.Center, frameStyle, content),
		c.helpViewer.ShortHelpView(c.activeBindings()),
	))
	v.AltScreen = true
	return v
}

func (c UI) resize(width, height int) UI {
	c.helpViewer.SetWidth(width)
	helpHeight := lipgloss.Height(c.helpViewer.ShortHelpView(c.keyMap.ShortHelp()))

	width -= frameStyle.Border.GetHorizontalBorderSize()
	height -= frameStyle.Border.GetVerticalBorderSize() + helpHeight

	c.pathPane = c.pathPane.Size(width, height)
	c.logsPane = c.logsPane.Size(width, height)
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
		Quit:       key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("ctr+c/q", "quit the program")),
		NextPane:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch between the path and logs")),
		ResetStats: key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "reset statistics")),
		TableKeys:  table.DefaultKeyMap(),
		StreamKeys: stream.DefaultKeyMap(),
	}
}
