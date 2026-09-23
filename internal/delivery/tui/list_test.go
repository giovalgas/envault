package tui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func visibleNames(l listModel) []string {
	names := make([]string, 0, len(l.visible))
	for _, idx := range l.visible {
		names = append(names, l.envs[idx].Name)
	}
	return names
}

func TestListFilterMatchesFields(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "nome", query: "stripe", want: []string{"stripe-test"}},
		{name: "descrição", query: "docker", want: []string{"postgres-local"}},
		{name: "tag", query: "cache", want: []string{"redis"}},
		{name: "chave sem diferenciar caixa", query: "pgpass", want: []string{"postgres-local"}},
		{name: "vazio mostra tudo", query: "", want: []string{"postgres-local", "redis", "stripe-test"}},
		{name: "sem resultado", query: "inexistente", want: []string{}},
		{name: "valor não é pesquisável", query: "s3cr3t", want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newListModel().setEnvs(seedEnvs(), "")
			l.filter.SetValue(tt.query)
			l = l.applyFilter()
			if got := visibleNames(l); !slices.Equal(got, tt.want) {
				t.Fatalf("visíveis = %v, esperado %v", got, tt.want)
			}
		})
	}
}

func TestListFilterFlow(t *testing.T) {
	sess := startSession(t, newTestVault(t), Options{})
	sess.typeText("/stripe")
	sess.waitFor("/ stripe")
	sess.press(tea.KeyEnter)
	m, out := sess.finish()

	if got := visibleNames(m.list); !slices.Equal(got, []string{"stripe-test"}) {
		t.Fatalf("visíveis = %v", got)
	}
	if m.list.filtering {
		t.Fatal("enter deveria sair do modo de filtro mantendo o filtro")
	}
	view := m.View()
	if strings.Contains(view, "postgres-local") || strings.Contains(view, "redis") {
		t.Fatalf("filtro não escondeu as demais envs:\n%s", view)
	}
	assertNoSecrets(t, "saída do filtro", out)
}

func TestListFilterEscClears(t *testing.T) {
	sess := startSession(t, newTestVault(t), Options{})
	sess.typeText("/redis")
	sess.waitFor("/ redis")
	sess.press(tea.KeyEsc)
	m, _ := sess.finish()
	if m.list.hasFilter() || len(m.list.visible) != 3 {
		t.Fatalf("esc deveria limpar o filtro, visíveis = %v", visibleNames(m.list))
	}
}

func TestListMarkToggle(t *testing.T) {
	sess := startSession(t, newTestVault(t), Options{})
	sess.typeText(" j ")
	sess.waitFor("2 marcadas")
	sess.typeText("k ")
	sess.waitFor("1 marcada")
	m, out := sess.finish()

	if !slices.Equal(m.list.marked, []string{"redis"}) {
		t.Fatalf("marcadas = %v, esperado [redis]", m.list.marked)
	}
	if !strings.Contains(m.View(), "[1]") {
		t.Fatal("view não mostra o marcador de seleção")
	}
	assertNoSecrets(t, "saída da marcação", out)
}

func TestListMarkedPrunedAfterReload(t *testing.T) {
	l := newListModel().setEnvs(seedEnvs(), "")
	l = l.toggleMark()
	l = l.move(1).toggleMark()
	envs := seedEnvs()[1:]
	l = l.setEnvs(envs, "")
	if !slices.Equal(l.marked, []string{"redis"}) {
		t.Fatalf("marcadas = %v, esperado [redis]", l.marked)
	}
	if env, ok := l.focused(); !ok || env.Name != "redis" {
		t.Fatalf("foco = %v, esperado redis", env.Name)
	}
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
	if got := visibleNames(m.list); !slices.Equal(got, []string{"redis", "stripe-test"}) {
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
	if len(m.list.envs) != 3 {
		t.Fatalf("lista = %v", visibleNames(m.list))
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
	if !copied.SameContent(original) {
		t.Fatal("cópia com conteúdo diferente")
	}
	if env, ok := m.list.focused(); !ok || env.Name != "postgres-local-copy" {
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

func TestScrollWindow(t *testing.T) {
	tests := []struct {
		cursor, total, size int
		start, end          int
	}{
		{cursor: 0, total: 3, size: 10, start: 0, end: 3},
		{cursor: 0, total: 20, size: 5, start: 0, end: 5},
		{cursor: 10, total: 20, size: 5, start: 8, end: 13},
		{cursor: 19, total: 20, size: 5, start: 15, end: 20},
		{cursor: 0, total: 0, size: 5, start: 0, end: 0},
	}
	for _, tt := range tests {
		start, end := scrollWindow(tt.cursor, tt.total, tt.size)
		if start != tt.start || end != tt.end {
			t.Errorf("scrollWindow(%d, %d, %d) = %d, %d; esperado %d, %d", tt.cursor, tt.total, tt.size, start, end, tt.start, tt.end)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in    string
		limit int
		want  string
	}{
		{in: "curto", limit: 10, want: "curto"},
		{in: "comprido demais", limit: 8, want: "comprid" + ellipsis},
		{in: "qualquer", limit: 0, want: ""},
	}
	for _, tt := range tests {
		if got := truncate(tt.in, tt.limit); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, esperado %q", tt.in, tt.limit, got, tt.want)
		}
	}
}

func TestListEmptyVault(t *testing.T) {
	l := newListModel().setEnvs(nil, "")
	view := l.view(defaultStyles(), termWidth, termHeight)
	if !strings.Contains(view, "nenhuma env no cofre") {
		t.Fatalf("view vazia inesperada:\n%s", view)
	}
	if _, ok := l.focused(); ok {
		t.Fatal("lista vazia não deveria ter foco")
	}
}
