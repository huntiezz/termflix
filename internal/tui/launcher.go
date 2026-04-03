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
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 60

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
			if m.cfg.FPS < 60 {
				m.cfg.FPS++
			}
		case key.Matches(msg, m.keys.FPSDn):
			if m.cfg.FPS > 1 {
				m.cfg.FPS--
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m launcherModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("termflix")
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Terminal video player (ffmpeg + Unicode renderers)")

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(72)

	line := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render

	body := strings.Builder{}
	body.WriteString(title)
	body.WriteString("\n")
	body.WriteString(sub)
	body.WriteString("\n\n")
	body.WriteString("Source:\n")
	body.WriteString(m.input.View())
	body.WriteString("\n\n")
	body.WriteString(fmt.Sprintf("Mode:  %s   Audio: %s   FPS: %d   Mute: %v\n",
		m.cfg.Mode, m.cfg.AudioEngine, m.cfg.FPS, m.cfg.Mute,
	))
	body.WriteString("\n")
	body.WriteString(line("Keys: enter=start  m=mode  a=audio  x=mute  +/-=fps  q=quit"))

	out := card.Render(body.String())
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, out)
	}
	return out
}

