package tui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/giovalgas/envault/internal/vault"
)

func openDetail(t *testing.T, opts Options) *session {
	t.Helper()
	sess := startSession(t, newTestVault(t), opts)
	sess.press(tea.KeyEnter)
	sess.waitFor("CHAVE")
	return sess
}

func TestDetailOpensMasked(t *testing.T) {
	sess := openDetail(t, Options{})
	m, out := sess.finish()

	if m.screen != screenDetail {
		t.Fatalf("tela = %v, esperado detalhe", m.screen)
	}
	view := m.View()
	for _, want := range []string{"postgres-local", "DATABASE_URL", "PGPASSWORD", maskedValue} {
		if !strings.Contains(view, want) {
			t.Errorf("detalhe não contém %q:\n%s", want, view)
		}
	}
	assertNoSecrets(t, "view do detalhe", view)
	assertNoSecrets(t, "saída do detalhe", out)
}

func TestDetailRevealOnlyFocusedRow(t *testing.T) {
	sess := openDetail(t, Options{})
	sess.typeText("v")
	sess.waitFor(secretDatabaseURL)
	m, _ := sess.finish()

	view := m.View()
	if !strings.Contains(view, secretDatabaseURL) {
		t.Fatalf("valor focado não foi revelado:\n%s", view)
	}
	assertNoSecrets(t, "linhas não focadas", view, secretPGPassword, secretStripeKey, secretRedisURL)
	if strings.Count(view, maskedValue) != 1 {
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
			sess := openDetail(t, Options{})
			sess.typeText(tt.keys)
			m, _ := sess.finish()
			view := m.View()
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

func TestDetailCopyUsesInjectedClipboard(t *testing.T) {
	var (
		mu     sync.Mutex
		copied []string
	)
	opts := Options{Clipboard: func(value string) error {
		mu.Lock()
		defer mu.Unlock()
		copied = append(copied, value)
		return nil
	}}
	sess := openDetail(t, opts)
	sess.typeText("jy")
	sess.waitFor("valor de PGPASSWORD copiado")
	m, out := sess.finish()

	mu.Lock()
	defer mu.Unlock()
	if len(copied) != 1 || copied[0] != secretPGPassword {
		t.Fatalf("clipboard recebeu %d valores, esperado só o de PGPASSWORD", len(copied))
	}
	assertNoSecrets(t, "saída da cópia", out)
	assertNoSecrets(t, "view da cópia", m.View())
}

func TestDetailCopyErrorDoesNotLeakValue(t *testing.T) {
	opts := Options{Clipboard: func(string) error { return errors.New("sem clipboard") }}
	sess := openDetail(t, opts)
	sess.typeText("y")
	sess.waitFor("copiar DATABASE_URL: sem clipboard")
	m, out := sess.finish()
	if !m.statusErr {
		t.Fatal("status deveria indicar erro")
	}
	assertNoSecrets(t, "saída do erro de cópia", out)
}

func TestDetailBackReturnsToList(t *testing.T) {
	for _, k := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyEsc}} {
		t.Run(k.String(), func(t *testing.T) {
			sess := openDetail(t, Options{})
			sess.tm.Send(k)
			sess.waitFor("stripe-test")
			m, _ := sess.finish()
			if m.screen != screenList {
				t.Fatalf("tela = %v, esperado lista", m.screen)
			}
		})
	}
}

func TestDetailRefreshAfterExternalChange(t *testing.T) {
	envs := seedEnvs()
	m := New(context.Background(), nil, Options{})
	m.list = m.list.setEnvs(envs, "")
	m.detail = newDetailModel(envs[0])
	m.screen = screenDetail

	changed := seedEnvs()
	changed[0].Vars = changed[0].Vars[:1]
	updated, _ := m.Update(envsLoadedMsg{envs: changed})
	got := updated.(Model)
	if got.screen != screenDetail || len(got.detail.env.Vars) != 1 {
		t.Fatalf("detalhe não foi atualizado: tela %v, %d chaves", got.screen, len(got.detail.env.Vars))
	}

	updated, _ = got.Update(envsLoadedMsg{envs: changed[1:]})
	if updated.(Model).screen != screenList {
		t.Fatal("env apagada deveria devolver para a lista")
	}
}

func TestDetailEmptyEnv(t *testing.T) {
	d := newDetailModel(vault.Env{Name: "vazia"})
	d = d.move(1).toggleReveal()
	if _, ok := d.focusedVar(); ok {
		t.Fatal("env vazia não tem linha focada")
	}
	if view := d.view(defaultStyles(), termWidth, termHeight); !strings.Contains(view, "env sem chaves") {
		t.Fatalf("view inesperada:\n%s", view)
	}
}
