package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	vault "github.com/giovalgas/envault/internal/vault/domain"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

func TestListFilterFlow(t *testing.T) {
	sess := startSession(t, newTestVault(t), Options{})
	sess.typeText("/stripe")
	sess.waitFor("/ stripe")
	sess.press(tea.KeyEnter)
	m, out := sess.finish()

	if got := m.list.VisibleNames(); !slices.Equal(got, []string{"stripe-test"}) {
		t.Fatalf("visíveis = %v", got)
	}
	if m.list.Filtering() {
		t.Fatal("enter deveria sair do modo de filtro mantendo o filtro")
	}
	view := m.View()
	if strings.Contains(view, "postgres-local") || strings.Contains(view, "redis") {
		t.Fatalf("filtro não escondeu as demais envs:\n%s", view)
	}
	assertNoSecrets(t, "saída do filtro", out)
}

func TestListMarkToggle(t *testing.T) {
	sess := startSession(t, newTestVault(t), Options{})
	sess.typeText(" j ")
	sess.waitFor("2 marcadas")
	sess.typeText("k ")
	sess.waitFor("1 marcada")
	m, out := sess.finish()

	if !slices.Equal(m.list.Marked(), []string{"redis"}) {
		t.Fatalf("marcadas = %v, esperado [redis]", m.list.Marked())
	}
	if !strings.Contains(m.View(), "[1]") {
		t.Fatal("view não mostra o marcador de seleção")
	}
	assertNoSecrets(t, "saída da marcação", out)
}

func TestListDeleteConfirmed(t *testing.T) {
	v := newTestVault(t)
	sess := startSession(t, v, Options{})
	sess.typeText("d")
	sess.waitFor("Apagar postgres-local")
	sess.typeText("postgres-local")
	sess.press(tea.KeyEnter)
	sess.waitFor("postgres-local apagada")
	m, out := sess.finish()

	if _, err := v.Get(context.Background(), "postgres-local"); !errors.Is(err, vault.ErrEnvNotFound) {
		t.Fatalf("env ainda existe no cofre: %v", err)
	}
	if got := m.list.VisibleNames(); !slices.Equal(got, []string{"redis", "stripe-test"}) {
		t.Fatalf("lista = %v", got)
	}
	assertNoSecrets(t, "saída do apagar", out)
}

func TestListDeleteWrongNameRefused(t *testing.T) {
	v := newTestVault(t)
	sess := startSession(t, v, Options{})
	sess.typeText("d")
	sess.waitFor("Apagar postgres-local")
	sess.typeText("b")
	sess.press(tea.KeyEnter)
	sess.waitFor(errNameMismatch.Error())
	sess.press(tea.KeyEsc)
	m, _ := sess.finish()

	if m.modal != nil {
		t.Fatal("esc deveria fechar o modal")
	}
	if _, err := v.Get(context.Background(), "postgres-local"); err != nil {
		t.Fatalf("env deveria permanecer: %v", err)
	}
	if m.list.Len() != 3 {
		t.Fatalf("lista = %v", m.list.VisibleNames())
	}
}

func TestListDuplicate(t *testing.T) {
	v := newTestVault(t)
	sess := startSession(t, v, Options{})
	sess.typeText("c")
	sess.waitFor("Duplicar postgres-local")
	sess.press(tea.KeyEnter)
	sess.waitFor("duplicada como postgres-local-copy")
	m, _ := sess.finish()

	ctx := context.Background()
	original, err := v.Get(ctx, "postgres-local")
	if err != nil {
		t.Fatalf("original: %v", err)
	}
	copied, err := v.Get(ctx, "postgres-local-copy")
	if err != nil {
		t.Fatalf("cópia: %v", err)
	}
	if !sameContent(copied, original) {
		t.Fatal("cópia com conteúdo diferente")
	}
	if env, ok := m.list.Focused(); !ok || env.Name != "postgres-local-copy" {
		t.Fatalf("foco deveria ir para a cópia, está em %q", env.Name)
	}
}

func TestListRename(t *testing.T) {
	v := newTestVault(t)
	sess := startSession(t, v, Options{})
	sess.typeText("r")
	sess.waitFor("Renomear postgres-local")
	sess.press(tea.KeyCtrlU)
	sess.typeText("pg")
	sess.press(tea.KeyEnter)
	sess.waitFor("renomeada para pg")
	sess.finish()

	ctx := context.Background()
	if _, err := v.Get(ctx, "pg"); err != nil {
		t.Fatalf("nova env: %v", err)
	}
	if _, err := v.Get(ctx, "postgres-local"); !errors.Is(err, vault.ErrEnvNotFound) {
		t.Fatalf("nome antigo ainda existe: %v", err)
	}
}

func TestListRenameInvalidNameKeepsModal(t *testing.T) {
	v := newTestVault(t)
	sess := startSession(t, v, Options{})
	sess.typeText("r")
	sess.waitFor("Renomear postgres-local")
	sess.press(tea.KeyCtrlU)
	sess.typeText("Nome Ruim")
	sess.press(tea.KeyEnter)
	sess.waitFor(vault.ErrInvalidName.Error())
	m, _ := sess.finish()

	if m.modal == nil {
		t.Fatal("modal deveria continuar aberto com nome inválido")
	}
	if _, err := v.Get(context.Background(), "postgres-local"); err != nil {
		t.Fatalf("env deveria permanecer: %v", err)
	}
}

func TestListRenameConflictShowsError(t *testing.T) {
	v := newTestVault(t)
	sess := startSession(t, v, Options{})
	sess.typeText("r")
	sess.waitFor("Renomear postgres-local")
	sess.press(tea.KeyCtrlU)
	sess.typeText("redis")
	sess.press(tea.KeyEnter)
	sess.waitFor(vault.ErrEnvExists.Error())
	m, _ := sess.finish()
	if !m.statusErr {
		t.Fatal("status deveria indicar erro")
	}
}

func TestListModalChoiceNavigation(t *testing.T) {
	v := newTestVault(t)
	sess := startSession(t, v, Options{})
	sess.typeText("d")
	sess.waitFor("Apagar postgres-local")
	sess.typeText("postgres-local")
	sess.press(tea.KeyTab)
	sess.press(tea.KeyEnter)
	m, _ := sess.finish()

	if m.modal != nil {
		t.Fatal("cancelar deveria fechar o modal")
	}
	if _, err := v.Get(context.Background(), "postgres-local"); err != nil {
		t.Fatalf("cancelar não deveria apagar: %v", err)
	}
}

func sameContent(a, b vaultusecase.EnvView) bool {
	return a.Description == b.Description && slices.Equal(a.Tags, b.Tags) && slices.Equal(a.Vars, b.Vars)
}
