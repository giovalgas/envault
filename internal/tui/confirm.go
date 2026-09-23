package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const modalMaxWidth = 64

type choice struct {
	label string
	run   func(input string) (tea.Cmd, error)
}

func cancelChoice(label string) choice {
	return choice{label: label}
}

type confirmModel struct {
	title    string
	body     []string
	hasInput bool
	input    textinput.Model
	choices  []choice
	focus    int
	err      string
}

func newConfirm(title string, body ...string) confirmModel {
	return confirmModel{title: title, body: body}
}

func (c confirmModel) withInput(prompt, value string) confirmModel {
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

func (c confirmModel) withChoices(choices ...choice) confirmModel {
	c.choices = choices
	c.focus = 0
	return c
}

func (c confirmModel) value() string {
	if !c.hasInput {
		return ""
	}
	return c.input.Value()
}

func (c confirmModel) update(msg tea.KeyMsg, keys keyMap) (confirmModel, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, keys.Back), msg.Type == tea.KeyCtrlC:
		return c, nil, true
	case !c.hasInput && key.Matches(msg, keys.Quit):
		return c, nil, true
	case key.Matches(msg, keys.Confirm):
		return c.selectFocused()
	case c.movesChoice(msg, keys.NextChoice):
		c.focus = (c.focus + 1) % max(len(c.choices), 1)
		return c, nil, false
	case c.movesChoice(msg, keys.PrevChoice):
		n := max(len(c.choices), 1)
		c.focus = (c.focus - 1 + n) % n
		return c, nil, false
	}
	if c.hasInput {
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(msg)
		c.err = ""
		return c, cmd, false
	}
	return c, nil, false
}

func (c confirmModel) movesChoice(msg tea.KeyMsg, binding key.Binding) bool {
	if !key.Matches(msg, binding) {
		return false
	}
	if c.hasInput {
		return msg.Type == tea.KeyTab || msg.Type == tea.KeyShiftTab
	}
	return true
}

func (c confirmModel) selectFocused() (confirmModel, tea.Cmd, bool) {
	if len(c.choices) == 0 {
		return c, nil, true
	}
	selected := c.choices[c.focus]
	if selected.run == nil {
		return c, nil, true
	}
	cmd, err := selected.run(c.value())
	if err != nil {
		c.err = err.Error()
		return c, nil, false
	}
	return c, cmd, true
}

func (c confirmModel) view(st styles, width int) string {
	inner := min(max(width-8, 20), modalMaxWidth)
	lines := []string{st.title.Render(truncate(c.title, inner))}
	for _, line := range c.body {
		lines = append(lines, lipgloss.NewStyle().Width(inner).Render(line))
	}
	if c.hasInput {
		c.input.Width = inner - lipgloss.Width(c.input.Prompt) - 1
		lines = append(lines, "", c.input.View())
	}
	if c.err != "" {
		lines = append(lines, "", st.errorText.Render(truncate(c.err, inner)))
	}
	if len(c.choices) > 0 {
		buttons := make([]string, len(c.choices))
		for i, ch := range c.choices {
			style := st.choice
			if i == c.focus {
				style = st.choiceActive
			}
			buttons[i] = style.Render(ch.label)
		}
		lines = append(lines, "", strings.Join(buttons, " "))
	}
	return st.modal.Render(strings.Join(lines, "\n"))
}
