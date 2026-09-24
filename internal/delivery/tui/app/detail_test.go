package app

import (
	"context"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/giovalgas/envault/internal/delivery/tui/screen/detail"
	"github.com/giovalgas/envault/internal/delivery/tui/viewmodel"
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
	for _, want := range []string{"postgres-local", "DATABASE_URL", "PGPASSWORD", viewmodel.MaskedValue} {
		if !strings.Contains(view, want) {
			t.Errorf("detalhe não contém %q:\n%s", want, view)
		}
	}
	assertNoSecrets(t, "view do detalhe", view)
	assertNoSecrets(t, "saída do detalhe", out)
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
	envs := envViews(seedEnvs())
	m := New(context.Background(), nil, Options{})
	m.list = m.list.SetEnvs(envs, "")
	m.detail = detail.New(envs[0], nil)
	m.screen = screenDetail

	changed := envViews(seedEnvs())
	changed[0].Vars = changed[0].Vars[:1]
	updated, _ := m.Update(envsLoadedMsg{envs: changed})
	got := updated.(Model)
	if got.screen != screenDetail || len(got.detail.Env().Vars) != 1 {
		t.Fatalf("detalhe não foi atualizado: tela %v, %d chaves", got.screen, len(got.detail.Env().Vars))
	}

	updated, _ = got.Update(envsLoadedMsg{envs: changed[1:]})
	if updated.(Model).screen != screenList {
		t.Fatal("env apagada deveria devolver para a lista")
	}
}
