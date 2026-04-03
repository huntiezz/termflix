package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type errorKeyMap struct {
	Quit key.Binding
}

func (k errorKeyMap) ShortHelp() []key.Binding { return []key.Binding{k.Quit} }
func (k errorKeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{{k.Quit}} }

func defaultErrorKeys() errorKeyMap {
	return errorKeyMap{
		Quit: key.NewBinding(key.WithKeys("enter", "q", "esc", "ctrl+c"), key.WithHelp("enter", "close")),
	}
}

type ErrorModel struct {
	err    error
	width  int
	height int
	keys   errorKeyMap
}

func NewErrorModel(err error) ErrorModel {
	return ErrorModel{err: err, keys: defaultErrorKeys()}
}

func RunError(ctx context.Context, err error) error {
	m := NewErrorModel(err)
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithAltScreen())
	_, runErr := p.Run()
	if runErr != nil {
		return runErr
	}
	return err
}

func (m ErrorModel) Init() tea.Cmd { return nil }

func (m ErrorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ErrorModel) View() string {
	bg := lipgloss.NewStyle().Background(lipgloss.Color("#000000")).Foreground(lipgloss.Color("#e5e7eb"))
	title := lipgloss.NewStyle().Bold(true).Render("termflix")
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af")).Render("Something went wrong")

	msg := "Unknown error"
	if m.err != nil {
		msg = m.err.Error()
	}
	msg = strings.TrimSpace(msg)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#262626")).
		Padding(1, 2).
		Width(88).
		Render(title + "\n" + sub + "\n\n" + msg + "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af")).Render("press enter to close"))

	out := bg.Render(box)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			out,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceBackground(lipgloss.Color("#000000")),
		)
	}
	return out
}

