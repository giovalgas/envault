package confirm

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/giovalgas/envault/internal/delivery/tui/theme"
)

const (
	waitTimeout = 5 * time.Second
	termWidth   = 100
	termHeight  = 24
)

var errMismatch = errors.New("o nome digitado não confere")

type chosenMsg struct {
	label string
	input string
}

type harness struct {
	modal  Model
	closed bool
	chosen chosenMsg
}

func (h harness) Init() tea.Cmd {
	return nil
}

func (h harness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chosenMsg:
		h.chosen = msg
		return h, tea.Quit
	case tea.KeyMsg:
		updated, cmd, closed := h.modal.Update(msg)
		h.modal = updated
		if closed {
			h.closed = true
			if cmd == nil {
				return h, tea.Quit
			}
		}
		return h, cmd
	}
	return h, nil
}

func (h harness) View() string {
	return h.modal.View(theme.DefaultStyles(), termWidth)
}

func deleteModal() Model {
	return New("Apagar a", "Digite a para confirmar.").
		WithInput("nome: ", "").
		WithChoices(
			Choice{Label: "apagar", Run: func(input string) (tea.Cmd, error) {
				if input != "a" {
					return nil, errMismatch
				}
				return func() tea.Msg { return chosenMsg{label: "apagar", input: input} }, nil
			}},
			Cancel(),
		)
}

func run(t *testing.T, modal Model, steps func(tm *teatest.TestModel)) harness {
	t.Helper()
	tm := teatest.NewTestModel(t, harness{modal: modal}, teatest.WithInitialTermSize(termWidth, termHeight))
	waitFor(t, tm, "Apagar a")
	steps(tm)
	final, ok := finalModel(t, tm).(harness)
	if !ok {
		t.Fatal("modelo final inesperado")
	}
	return final
}

func waitFor(t *testing.T, tm *teatest.TestModel, text string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(text))
	}, teatest.WithDuration(waitTimeout), teatest.WithCheckInterval(10*time.Millisecond))
}

func TestConfirmRunsChoiceWithInput(t *testing.T) {
	h := run(t, deleteModal(), func(tm *teatest.TestModel) {
		tm.Type("a")
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	})
	if !h.closed || h.chosen.label != "apagar" || h.chosen.input != "a" {
		t.Fatalf("closed %v chosen %+v", h.closed, h.chosen)
	}
}

func TestConfirmErrorKeepsModalOpen(t *testing.T) {
	h := run(t, deleteModal(), func(tm *teatest.TestModel) {
		tm.Type("b")
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
		waitFor(t, tm, errMismatch.Error())
		if err := tm.Quit(); err != nil {
			t.Fatalf("encerrar: %v", err)
		}
	})
	if h.closed || h.chosen.label != "" {
		t.Fatalf("modal deveria continuar aberto: closed %v chosen %+v", h.closed, h.chosen)
	}
	if !strings.Contains(h.View(), errMismatch.Error()) {
		t.Fatalf("erro não aparece no modal:\n%s", h.View())
	}
}

func TestConfirmTabMovesToCancel(t *testing.T) {
	h := run(t, deleteModal(), func(tm *teatest.TestModel) {
		tm.Type("a")
		tm.Send(tea.KeyMsg{Type: tea.KeyTab})
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	})
	if !h.closed || h.chosen.label != "" {
		t.Fatalf("cancelar deveria fechar sem executar: closed %v chosen %+v", h.closed, h.chosen)
	}
}

func TestConfirmEscCloses(t *testing.T) {
	h := run(t, deleteModal(), func(tm *teatest.TestModel) {
		tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	})
	if !h.closed {
		t.Fatal("esc deveria fechar o modal")
	}
}

func TestConfirmWithoutInputQuitClosesAndArrowsCycle(t *testing.T) {
	modal := New("Apagar a").WithChoices(Choice{Label: "sim"}, Choice{Label: "não"}, Cancel())
	modal, _, _ = modal.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if modal.focus != 2 {
		t.Fatalf("left deveria voltar para a última opção, foco %d", modal.focus)
	}
	modal, _, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRight})
	if modal.focus != 0 {
		t.Fatalf("right deveria dar a volta, foco %d", modal.focus)
	}
	if _, _, closed := modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); !closed {
		t.Fatal("q sem campo de texto deveria fechar")
	}
	withInput := deleteModal()
	if _, _, closed := withInput.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); closed {
		t.Fatal("q com campo de texto é digitação, não fecha")
	}
}

func finalModel(t *testing.T, tm *teatest.TestModel) tea.Model {
	t.Helper()
	final := tm.FinalModel(t, teatest.WithFinalTimeout(waitTimeout))
	for deadline := time.Now().Add(waitTimeout); final == nil && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
		final = tm.FinalModel(t)
	}
	return final
}
