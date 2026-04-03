package tui

import (
	"context"
	"fmt"
	"math"
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
	bg := lipgloss.NewStyle().Background(lipgloss.Color("#0b0f14")).Foreground(lipgloss.Color("#e5e7eb"))

	logo := renderGradientASCII(termflixASCII, "#64748b", "#f8fafc")
	sub := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94a3b8")).
		Render("Watch videos in your terminal. Paste a file path, YouTube URL, or '-' for stdin.")

	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).Render("│")
	inputBox := lipgloss.NewStyle().
		Background(lipgloss.Color("#111827")).
		Foreground(lipgloss.Color("#e5e7eb")).
		Padding(0, 1).
		Width(74).
		Render(accent + " " + m.input.View())

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8")).Render
	kv := lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1")).Render

	fpsLabel := kv("source")
	if m.cfg.FPS > 0 {
		fpsLabel = kv(fmt.Sprintf("cap %d", m.cfg.FPS))
	}

	meta := hint(fmt.Sprintf("mode %s   audio %s   fps %s   mute %v", kv(string(m.cfg.Mode)), kv(string(m.cfg.AudioEngine)), fpsLabel, m.cfg.Mute))
	footer := hint("enter start    m mode    a audio    x mute    +/- fps-cap    q quit")

	body := strings.Builder{}
	body.WriteString(logo)
	body.WriteString("\n\n")
	body.WriteString(sub)
	body.WriteString("\n\n")
	body.WriteString(inputBox)
	body.WriteString("\n\n")
	body.WriteString(meta)
	body.WriteString("\n\n")
	body.WriteString(footer)

	out := bg.Render(body.String())
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, out)
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

func renderGradientASCII(ascii, startHex, endHex string) string {
	lines := strings.Split(ascii, "\n")
	// Find max width for consistent gradient mapping.
	maxW := 0
	for _, ln := range lines {
		if w := lipgloss.Width(ln); w > maxW {
			maxW = w
		}
	}
	if maxW == 0 {
		return ""
	}

	var b strings.Builder
	for li, ln := range lines {
		_ = li
		runes := []rune(ln)
		for i, r := range runes {
			if r == ' ' {
				b.WriteRune(r)
				continue
			}
			t := float64(i) / float64(max(1, maxW-1))
			c := lerpHex(startHex, endHex, t)
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(string(r)))
		}
		if li != len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func lerpHex(a, b string, t float64) string {
	ar, ag, ab := hexToRGB(a)
	br, bg, bb := hexToRGB(b)
	t = math.Max(0, math.Min(1, t))
	r := uint8(math.Round(float64(ar) + (float64(br)-float64(ar))*t))
	g := uint8(math.Round(float64(ag) + (float64(bg)-float64(ag))*t))
	bl := uint8(math.Round(float64(ab) + (float64(bb)-float64(ab))*t))
	return fmt.Sprintf("#%02x%02x%02x", r, g, bl)
}

func hexToRGB(s string) (uint8, uint8, uint8) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return 255, 255, 255
	}
	var v uint64
	_, _ = fmt.Sscanf(s, "%06x", &v)
	return uint8(v >> 16), uint8((v >> 8) & 0xff), uint8(v & 0xff)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
