package detail

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/giovalgas/envault/internal/delivery/tui/theme"
	"github.com/giovalgas/envault/internal/delivery/tui/viewmodel"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	waitTimeout = 5 * time.Second
	termWidth   = 120
	termHeight  = 32

	secretDatabaseURL = "valor-database"
	secretPGPassword  = "valor-password"
)

var allSecrets = []string{secretDatabaseURL, secretPGPassword}

func postgres() vaultusecase.EnvView {
	return vaultusecase.EnvView{
		Name:        "postgres-local",
		Description: "Postgres local via docker-compose",
		Tags:        []string{"db", "local"},
		Vars: []vaultusecase.VarView{
			{Key: "DATABASE_URL", Value: secretDatabaseURL},
			{Key: "PGPASSWORD", Value: secretPGPassword},
		},
	}
}

type harness struct {
	detail Model
	status string
	err    error
	last   Intent
}

func (h harness) Init() tea.Cmd {
	return nil
}

func (h harness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CopiedMsg:
		h.status, h.err = msg.Status, msg.Err
	case tea.KeyMsg:
		updated, cmd, intent := h.detail.Update(msg)
		h.detail, h.last = updated, intent
		if intent == IntentBack {
			return h, tea.Quit
		}
		return h, cmd
	}
	return h, nil
}

func (h harness) View() string {
	status := h.status
	if h.err != nil {
		status = h.err.Error()
	}
	return h.detail.View(theme.DefaultStyles(), termWidth, termHeight-1) + "\n" + status
}

type session struct {
	t  *testing.T
	tm *teatest.TestModel
}

func open(t *testing.T, write func(string) error) *session {
	t.Helper()
	tm := teatest.NewTestModel(t, harness{detail: New(postgres(), write)}, teatest.WithInitialTermSize(termWidth, termHeight))
	sess := &session{t: t, tm: tm}
	sess.waitFor("CHAVE")
	return sess
}

func (s *session) waitFor(text string) {
	s.t.Helper()
	teatest.WaitFor(s.t, s.tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(text))
	}, teatest.WithDuration(waitTimeout), teatest.WithCheckInterval(10*time.Millisecond))
}

func (s *session) finish() (harness, string) {
	s.t.Helper()
	if err := s.tm.Quit(); err != nil {
		s.t.Fatalf("encerrar programa: %v", err)
	}
	return s.collect()
}

func (s *session) collect() (harness, string) {
	s.t.Helper()
	final, ok := finalModel(s.t, s.tm).(harness)
	if !ok {
		s.t.Fatal("modelo final inesperado")
	}
	out, err := io.ReadAll(s.tm.FinalOutput(s.t, teatest.WithFinalTimeout(waitTimeout)))
	if err != nil {
		s.t.Fatalf("ler saída: %v", err)
	}
	return final, string(out)
}

func noClipboard(string) error {
	return errors.New("clipboard indisponível no teste")
}

func assertNoSecrets(t *testing.T, where, text string, secrets ...string) {
	t.Helper()
	if len(secrets) == 0 {
		secrets = allSecrets
	}
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Fatalf("%s contém o valor %q", where, secret)
		}
	}
}

func TestDetailOpensMasked(t *testing.T) {
	sess := open(t, noClipboard)
	h, out := sess.finish()
	view := h.View()
	for _, want := range []string{"postgres-local", "DATABASE_URL", "PGPASSWORD", viewmodel.MaskedValue, "db, local"} {
		if !strings.Contains(view, want) {
			t.Errorf("detalhe não contém %q:\n%s", want, view)
		}
	}
	assertNoSecrets(t, "view do detalhe", view)
	assertNoSecrets(t, "saída do detalhe", out)
}

func TestDetailRevealOnlyFocusedRow(t *testing.T) {
	sess := open(t, noClipboard)
	sess.tm.Type("v")
	sess.waitFor(secretDatabaseURL)
	h, _ := sess.finish()

	view := h.View()
	if !strings.Contains(view, secretDatabaseURL) {
		t.Fatalf("valor focado não foi revelado:\n%s", view)
	}
	assertNoSecrets(t, "linhas não focadas", view, secretPGPassword)
	if strings.Count(view, viewmodel.MaskedValue) != 1 {
		t.Fatalf("esperada exatamente uma linha mascarada:\n%s", view)
	}
}

func TestDetailRevealToggleAndMoveHides(t *testing.T) {
	tests := []struct {
		name  string
		keys  string
		wants string
	}{
		{name: "v duas vezes oculta", keys: "vv"},
		{name: "mover oculta", keys: "vj"},
		{name: "revela a segunda linha", keys: "vjv", wants: secretPGPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := open(t, noClipboard)
			sess.tm.Type(tt.keys)
			h, _ := sess.finish()
			view := h.View()
			var hidden []string
			for _, s := range allSecrets {
				if s != tt.wants {
					hidden = append(hidden, s)
				}
			}
			assertNoSecrets(t, "detalhe", view, hidden...)
			if tt.wants != "" && !strings.Contains(view, tt.wants) {
				t.Fatalf("valor %q deveria estar visível:\n%s", tt.wants, view)
			}
		})
	}
}

func TestDetailCopyWritesOnlyFocusedValue(t *testing.T) {
	var (
		mu     sync.Mutex
		copied []string
	)
	write := func(value string) error {
		mu.Lock()
		defer mu.Unlock()
		copied = append(copied, value)
		return nil
	}
	sess := open(t, write)
	sess.tm.Type("jy")
	sess.waitFor("valor de PGPASSWORD copiado")
	h, out := sess.finish()

	mu.Lock()
	defer mu.Unlock()
	if len(copied) != 1 || copied[0] != secretPGPassword {
		t.Fatalf("clipboard recebeu %d valores, esperado só o de PGPASSWORD", len(copied))
	}
	assertNoSecrets(t, "saída da cópia", out)
	assertNoSecrets(t, "view da cópia", h.View())
}

func TestDetailCopyErrorDoesNotLeakValue(t *testing.T) {
	sess := open(t, func(string) error { return errors.New("sem clipboard") })
	sess.tm.Type("y")
	sess.waitFor("copiar DATABASE_URL: sem clipboard")
	h, out := sess.finish()
	if h.err == nil {
		t.Fatal("cópia deveria devolver erro")
	}
	assertNoSecrets(t, "saída do erro de cópia", out)
}

func TestDetailBackAndHelpIntents(t *testing.T) {
	for _, k := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyEsc}} {
		t.Run(k.String(), func(t *testing.T) {
			sess := open(t, noClipboard)
			sess.tm.Send(k)
			h, _ := sess.collect()
			if h.last != IntentBack {
				t.Fatalf("intent = %v, esperado voltar", h.last)
			}
		})
	}
	_, _, intent := New(postgres(), noClipboard).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if intent != IntentHelp {
		t.Fatalf("? deveria pedir ajuda, intent = %v", intent)
	}
}

func TestDetailRefresh(t *testing.T) {
	d := New(postgres(), noClipboard).move(1).toggleReveal()
	changed := postgres()
	changed.Vars = changed.Vars[:1]
	refreshed, ok := d.Refresh([]vaultusecase.EnvView{changed})
	if !ok || len(refreshed.Env().Vars) != 1 || refreshed.cursor != 0 || refreshed.revealed {
		t.Fatalf("refresh = ok %v, %d chaves, cursor %d, revelado %v", ok, len(refreshed.Env().Vars), refreshed.cursor, refreshed.revealed)
	}
	if _, ok := d.Refresh(nil); ok {
		t.Fatal("env apagada não deveria continuar no detalhe")
	}
}

func TestDetailEmptyEnv(t *testing.T) {
	d := New(vaultusecase.EnvView{Name: "vazia"}, noClipboard)
	d = d.move(1).toggleReveal()
	if _, ok := d.focusedVar(); ok {
		t.Fatal("env vazia não tem linha focada")
	}
	if view := d.View(theme.DefaultStyles(), termWidth, termHeight); !strings.Contains(view, "env sem chaves") {
		t.Fatalf("view inesperada:\n%s", view)
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
