package confirm

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/giovalgas/envault/internal/delivery/tui/theme"
)

const (
	modalMaxWidth = 64
	cancelLabel   = "cancelar"
)

type Choice struct {
	Label string
	Run   func(input string) (tea.Cmd, error)
}

func Cancel() Choice {
	return Choice{Label: cancelLabel}
}

type Model struct {
	keys     theme.KeyMap
	title    string
	body     []string
	hasInput bool
	input    textinput.Model
	choices  []Choice
	focus    int
	err      string
	dismiss  func() tea.Cmd
}

func New(title string, body ...string) Model {
	return Model{keys: theme.DefaultKeyMap(), title: title, body: body}
}

func (c Model) WithInput(prompt, value string) Model {
	ti := textinput.New()
	ti.Prompt = prompt
	ti.SetValue(value)
	ti.CharLimit = 128
	ti.Cursor.SetMode(cursor.CursorStatic)
	ti.Focus()
	c.input = ti
	c.hasInput = true
	return c
}

func (c Model) WithChoices(choices ...Choice) Model {
	c.choices = choices
	c.focus = 0
	return c
}

func (c Model) WithDismiss(dismiss func() tea.Cmd) Model {
	c.dismiss = dismiss
	return c
}

func (c Model) dismissCmd() tea.Cmd {
	if c.dismiss == nil {
		return nil
	}
	return c.dismiss()
}

func (c Model) value() string {
	if !c.hasInput {
		return ""
	}
	return c.input.Value()
}

func (c Model) Update(msg tea.KeyMsg) (updated Model, cmd tea.Cmd, closed bool) {
	switch {
	case key.Matches(msg, c.keys.Back), msg.Type == tea.KeyCtrlC:
		return c, c.dismissCmd(), true
	case !c.hasInput && key.Matches(msg, c.keys.Quit):
		return c, c.dismissCmd(), true
	case key.Matches(msg, c.keys.Confirm):
		return c.selectFocused()
	case c.movesChoice(msg, c.keys.NextChoice):
		c.focus = (c.focus + 1) % max(len(c.choices), 1)
		return c, nil, false
	case c.movesChoice(msg, c.keys.PrevChoice):
		n := max(len(c.choices), 1)
		c.focus = (c.focus - 1 + n) % n
		return c, nil, false
	}
	if c.hasInput {
		c.input, cmd = c.input.Update(msg)
		c.err = ""
		return c, cmd, false
	}
	return c, nil, false
}

func (c Model) movesChoice(msg tea.KeyMsg, binding key.Binding) bool {
	if !key.Matches(msg, binding) {
		return false
	}
	if c.hasInput {
		return msg.Type == tea.KeyTab || msg.Type == tea.KeyShiftTab
	}
	return true
}

func (c Model) selectFocused() (Model, tea.Cmd, bool) {
	if len(c.choices) == 0 {
		return c, nil, true
	}
	selected := c.choices[c.focus]
	if selected.Run == nil {
		return c, nil, true
	}
	cmd, err := selected.Run(c.value())
	if err != nil {
		c.err = err.Error()
		return c, nil, false
	}
	return c, cmd, true
}

func (c Model) View(st theme.Styles, width int) string {
	inner := min(max(width-8, 20), modalMaxWidth)
	lines := []string{st.Title.Render(theme.Truncate(c.title, inner))}
	for _, line := range c.body {
		lines = append(lines, lipgloss.NewStyle().Width(inner).Render(line))
	}
	if c.hasInput {
		c.input.Width = inner - lipgloss.Width(c.input.Prompt) - 1
		lines = append(lines, "", c.input.View())
	}
	if c.err != "" {
		lines = append(lines, "", st.ErrorText.Render(theme.Truncate(c.err, inner)))
	}
	if len(c.choices) > 0 {
		buttons := make([]string, len(c.choices))
		for i, ch := range c.choices {
			style := st.Choice
			if i == c.focus {
				style = st.ChoiceActive
			}
			buttons[i] = style.Render(ch.Label)
		}
		lines = append(lines, "", strings.Join(buttons, " "))
	}
	return st.Modal.Render(strings.Join(lines, "\n"))
}
