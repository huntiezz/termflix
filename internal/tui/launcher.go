package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/huntiezz/termflix/internal/util"
)

type launcherKeyMap struct {
	Quit  key.Binding
	Start key.Binding
	Mode  key.Binding
	Audio key.Binding
	Mute  key.Binding
	FPSUp key.Binding
	FPSDn key.Binding
}

func defaultLauncherKeys() launcherKeyMap {
	return launcherKeyMap{
		Quit:  key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
		Start: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "start")),
		Mode:  key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mode")),
		Audio: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "audio")),
		Mute:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "mute")),
		FPSUp: key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "fps+")),
		FPSDn: key.NewBinding(key.WithKeys("-", "_"), key.WithHelp("-", "fps-")),
	}
}

type LauncherResult struct {
	Config  util.Config
	Started bool
}

func RunLauncher(ctx context.Context, cfg util.Config) (LauncherResult, error) {
	m := newLauncherModel(cfg)
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithAltScreen())
	res, err := p.Run()
	if err != nil {
		return LauncherResult{}, err
	}
	final := res.(launcherModel)
	return LauncherResult{Config: final.cfg, Started: final.started}, nil
}

type launcherModel struct {
	cfg util.Config

	input textinput.Model
	keys  launcherKeyMap

	width  int
	height int

	started bool
}

func newLauncherModel(cfg util.Config) launcherModel {
	ti := textinput.New()
	ti.Placeholder = "video file path, YouTube URL, or '-' for stdin"
	ti.Prompt = ""
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 66
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb"))
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb"))

	return launcherModel{
		cfg:   cfg,
		input: ti,
		keys:  defaultLauncherKeys(),
	}
}

func (m launcherModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m launcherModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Start):
			in := strings.TrimSpace(m.input.Value())
			if in == "" {
				return m, nil
			}
			m.cfg.Input = in
			m.started = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Mode):
			switch m.cfg.Mode {
			case util.RenderModeBlocks:
				m.cfg.Mode = util.RenderModeBraille
			case util.RenderModeBraille:
				m.cfg.Mode = util.RenderModeASCII
			default:
				m.cfg.Mode = util.RenderModeBlocks
			}
		case key.Matches(msg, m.keys.Audio):
			switch m.cfg.AudioEngine {
			case util.AudioEngineNone:
				m.cfg.AudioEngine = util.AudioEngineMPV
			case util.AudioEngineMPV:
				m.cfg.AudioEngine = util.AudioEngineFFPlay
			default:
				m.cfg.AudioEngine = util.AudioEngineNone
			}
		case key.Matches(msg, m.keys.Mute):
			m.cfg.Mute = !m.cfg.Mute
		case key.Matches(msg, m.keys.FPSUp):
			if m.cfg.FPS < 240 {
				m.cfg.FPS++
			}
		case key.Matches(msg, m.keys.FPSDn):
			if m.cfg.FPS > 0 {
				m.cfg.FPS--
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m launcherModel) View() string {
	const panelW = 80

	// Neutral grayscale palette (avoid blue-tinted backgrounds).
	bgColor := lipgloss.Color("#000000")
	fg := lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb"))

	center := lipgloss.NewStyle().Width(panelW).Align(lipgloss.Center)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af"))
	kv := lipgloss.NewStyle().Foreground(lipgloss.Color("#d1d5db"))

	// Flat logo (no gradients, no colored accents).
	logo := center.Render(termflixASCII)
	sub := center.Render(muted.Render("Paste a file path, YouTube URL, or '-' for stdin."))

	// Opencode-like: simple dark input bar.
	inputBarStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#141414")).
		Foreground(lipgloss.Color("#e5e7eb")).
		Padding(0, 2).
		MarginTop(0).
		Width(panelW - 6)
	// Force a hard reset after the bar so its background doesn't "bleed"
	// into following lines on some Windows terminals.
	inputBar := center.Render(inputBarStyle.Render(strings.TrimRight(m.input.View(), " ")) + "\x1b[0m")

	fpsLabel := kv.Render("source")
	if m.cfg.FPS > 0 {
		fpsLabel = kv.Render(fmt.Sprintf("cap %d", m.cfg.FPS))
	}

	meta := center.Render(muted.Render(fmt.Sprintf("mode %s   audio %s   fps %s   mute %v",
		kv.Render(string(m.cfg.Mode)),
		kv.Render(string(m.cfg.AudioEngine)),
		fpsLabel,
		m.cfg.Mute,
	)))

	footer := center.Render(muted.Render("enter start    q quit    m mode    a audio    x mute    +/- fps-cap"))

	body := strings.Builder{}
	body.WriteString(logo)
	body.WriteString("\n\n")
	body.WriteString(sub)
	body.WriteString("\n\n")
	body.WriteString(inputBar)
	body.WriteString("\n\n")
	body.WriteString(meta)
	body.WriteString("\n\n")
	body.WriteString(footer)

	out := fg.Render(body.String())
	if m.width > 0 && m.height > 0 {
		// Fill the entire viewport with background color, then center the panel.
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			out,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceBackground(bgColor),
		)
	}
	return out
}

const termflixASCII = `
████████╗███████╗██████╗ ███╗   ███╗███████╗██╗     ██╗██╗  ██╗
╚══██╔══╝██╔════╝██╔══██╗████╗ ████║██╔════╝██║     ██║╚██╗██╔╝
   ██║   █████╗  ██████╔╝██╔████╔██║█████╗  ██║     ██║ ╚███╔╝ 
   ██║   ██╔══╝  ██╔══██╗██║╚██╔╝██║██╔══╝  ██║     ██║ ██╔██╗ 
   ██║   ███████╗██║  ██║██║ ╚═╝ ██║██║     ███████╗██║██╔╝ ██╗
   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝╚═╝     ╚══════╝╚═╝╚═╝  ╚═╝`
