package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/huntiezz/termflix/internal/player"
	"github.com/huntiezz/termflix/internal/render"
	"github.com/huntiezz/termflix/internal/util"
)

type keyMap struct {
	TogglePause key.Binding
	Quit        key.Binding
	SeekBack    key.Binding
	SeekForward key.Binding
	SeekBackBig key.Binding
	SeekFwdBig  key.Binding
	CycleMode   key.Binding
	ToggleFit   key.Binding
	ToggleMute  key.Binding
	ToggleHelp  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.TogglePause, k.SeekBack, k.SeekForward, k.CycleMode, k.Quit, k.ToggleHelp}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.TogglePause, k.Quit, k.ToggleHelp},
		{k.SeekBack, k.SeekForward, k.SeekBackBig, k.SeekFwdBig},
		{k.CycleMode, k.ToggleFit, k.ToggleMute},
	}
}

func defaultKeyMap() keyMap {
	return keyMap{
		TogglePause: key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "pause/resume")),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		SeekBack:    key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "seek -5s")),
		SeekForward: key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "seek +5s")),
		SeekBackBig: key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "seek -10s")),
		SeekFwdBig:  key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "seek +10s")),
		CycleMode:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "cycle render")),
		ToggleFit:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "fit/fill")),
		ToggleMute:  key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mute")),
		ToggleHelp:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
	}
}

type Config struct {
	ShowUI       bool
	InitialMode  util.RenderMode
	FitMode      util.FitMode
	Title        string
	Duration     time.Duration
	InitialMuted bool
	FPS          int
	AudioEngine  util.AudioEngine
}

type Model struct {
	ctx    context.Context
	player *player.Player
	cfg    Config

	width  int
	height int

	keys keyMap
	help help.Model

	showHelp bool

	err error

	lastSeq  uint64
	lastMode util.RenderMode
	lastBody string
}

func NewModel(p *player.Player, cfg Config) Model {
	h := help.New()
	h.ShowAll = true
	return Model{
		player: p,
		cfg:    cfg,
		keys:   defaultKeyMap(),
		help:   h,
	}
}

type Result struct {
	Err error
}

// RunProgram starts the Bubble Tea program.
func RunProgram(ctx context.Context, m Model) (Result, error) {
	m.ctx = ctx
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithAltScreen())
	res, err := p.Run()
	if err != nil {
		return Result{Err: err}, err
	}
	if final, ok := res.(Model); ok {
		return Result{Err: final.err}, nil
	}
	return Result{}, nil
}

type redrawMsg struct{}

func redrawTick() tea.Cmd {
	// 60Hz UI cadence feels smoother and reduces "black flash" on some terminals.
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg { return redrawMsg{} })
}

// Init starts the player.
func (m Model) Init() tea.Cmd {
	return redrawTick()
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.player.Stop()
			return m, tea.Quit
		case key.Matches(msg, m.keys.TogglePause):
			m.player.TogglePause()
		case key.Matches(msg, m.keys.SeekBack):
			m.player.Seek(-5 * time.Second)
		case key.Matches(msg, m.keys.SeekForward):
			m.player.Seek(5 * time.Second)
		case key.Matches(msg, m.keys.SeekBackBig):
			m.player.Seek(-10 * time.Second)
		case key.Matches(msg, m.keys.SeekFwdBig):
			m.player.Seek(10 * time.Second)
		case key.Matches(msg, m.keys.CycleMode):
			m.player.CycleMode()
		case key.Matches(msg, m.keys.ToggleFit):
			m.player.ToggleFit()
		case key.Matches(msg, m.keys.ToggleMute):
			m.player.ToggleMute()
		case key.Matches(msg, m.keys.ToggleHelp):
			m.showHelp = !m.showHelp
		}
		return m, redrawTick()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.ctx != nil {
			// Reserve 1 line for the status bar.
			go m.player.Start(m.ctx, m.width, m.height-1)
		}
		return m, redrawTick()
	case redrawMsg:
		return m, redrawTick()
	}
	return m, nil
}

// View renders current frame and status bar.
func (m Model) View() string {
	st := m.player.Snapshot()

	mode := render.ModeBlocks
	switch st.Mode {
	case util.RenderModeBlocks:
		mode = render.ModeBlocks
	case util.RenderModeBraille:
		mode = render.ModeBraille
	case util.RenderModeASCII:
		mode = render.ModeASCII
	}

	var body string
	if m.showHelp {
		body = m.help.View(m.keys)
	} else if st.Frame.Cols > 0 && st.Frame.Rows > 0 {
		// Cache rendered output so we don't re-render the same frame on every tick.
		if st.Seq == m.lastSeq && st.Mode == m.lastMode && m.lastBody != "" {
			body = m.lastBody
		} else {
			body = render.Render(st.Frame, mode)
			body = padToViewport(body, m.width, max(0, m.height-1))
			m.lastSeq = st.Seq
			m.lastMode = st.Mode
			m.lastBody = body
		}
	} else {
		body = padToViewport("", m.width, max(0, m.height-1))
	}

	status := m.renderStatus(st)

	return body + "\n" + status
}

func padToViewport(body string, width, lines int) string {
	if width <= 0 || lines <= 0 {
		return body
	}
	parts := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(parts) == 1 && parts[0] == "" {
		parts = []string{}
	}
	// pad/truncate lines
	out := make([]string, 0, lines)
	for i := 0; i < lines; i++ {
		ln := ""
		if i < len(parts) {
			ln = parts[i]
		}
		// Ensure the terminal line is fully cleared to avoid artifacts/flicker.
		// \x1b[K clears from cursor to end of line.
		w := lipgloss.Width(ln)
		if w < width {
			ln = ln + strings.Repeat(" ", width-w)
		}
		out = append(out, "\x1b[0m"+ln+"\x1b[K")
	}
	return strings.Join(out, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) renderStatus(st player.State) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("57")).
		PaddingLeft(1).
		PaddingRight(1)

	mode := string(st.Mode)
	if mode == "" {
		mode = "blocks"
	}

	audioLabel := string(m.cfg.AudioEngine)
	if audioLabel == "" {
		audioLabel = "none"
	}
	if st.Muted {
		audioLabel = "muted"
	} else if m.cfg.AudioEngine == util.AudioEngineNone {
		audioLabel = "silent"
	}
	paused := ""
	if st.Paused {
		paused = "paused"
	}

	left := fmt.Sprintf(" %s ", m.cfg.Title)
	center := fmt.Sprintf("%s/%s  %s  %.1ffps", formatDuration(st.Position), formatDuration(st.Duration), mode, st.FPSActual)
	right := fmt.Sprintf("%s %s", audioLabel, paused)
	if st.Err != "" {
		right = "error"
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, left, center, right)
	if m.width > 0 {
		content = lipgloss.PlaceHorizontal(m.width, lipgloss.Left, content)
	}
	return style.Render(content)
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "00:00"
	}
	secs := int(d.Seconds())
	m := secs / 60
	s := secs % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

