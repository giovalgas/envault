package list

import (
	"bytes"
	"io"
	"slices"
	"strings"
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
)

var secrets = []string{"valor-database", "valor-password", "valor-stripe", "valor-redis"}

func seedEnvs() []vaultusecase.EnvView {
	return []vaultusecase.EnvView{
		{
			Name:        "postgres-local",
			Description: "Postgres local via docker-compose",
			Tags:        []string{"db", "local"},
			Vars: []vaultusecase.VarView{
				{Key: "DATABASE_URL", Value: secrets[0]},
				{Key: "PGPASSWORD", Value: secrets[1]},
			},
		},
		{
			Name:        "redis",
			Description: "Cache",
			Tags:        []string{"cache"},
			Vars:        []vaultusecase.VarView{{Key: "REDIS_URL", Value: secrets[3]}},
		},
		{
			Name:        "stripe-test",
			Description: "Stripe modo teste",
			Tags:        []string{"payments"},
			Vars:        []vaultusecase.VarView{{Key: "STRIPE_SECRET_KEY", Value: secrets[2]}},
		},
	}
}

type harness struct {
	list Model
	last Intent
}

func (h harness) Init() tea.Cmd {
	return nil
}

func (h harness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return h, nil
	}
	updated, cmd, intent := h.list.Update(keyMsg)
	h.list, h.last = updated, intent
	if intent == IntentQuit {
		return h, tea.Quit
	}
	return h, cmd
}

func (h harness) View() string {
	st := theme.DefaultStyles()
	return strings.Join([]string{"envault", h.list.SelectionView(st, termWidth), h.list.View(st, termWidth, termHeight-2)}, "\n")
}

type session struct {
	t  *testing.T
	tm *teatest.TestModel
}

func start(t *testing.T) *session {
	t.Helper()
	h := harness{list: New().SetEnvs(seedEnvs(), "")}
	tm := teatest.NewTestModel(t, h, teatest.WithInitialTermSize(termWidth, termHeight))
	sess := &session{t: t, tm: tm}
	sess.waitFor("postgres-local")
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

func assertNoSecrets(t *testing.T, where, text string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Fatalf("%s contém o valor %q", where, secret)
		}
	}
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
			l := New().SetEnvs(seedEnvs(), "")
			l.filter.SetValue(tt.query)
			l = l.applyFilter()
			if got := l.VisibleNames(); !slices.Equal(got, tt.want) {
				t.Fatalf("visíveis = %v, esperado %v", got, tt.want)
			}
		})
	}
}

func TestListRendersMasked(t *testing.T) {
	sess := start(t)
	h, out := sess.finish()
	view := h.View()
	for _, want := range []string{
		"postgres-local", "redis", "stripe-test", "DATABASE_URL", "PGPASSWORD", viewmodel.MaskedValue,
		"Postgres local via docker-compose", "db, local", "2 chaves",
		"SELECTED_ENVS=",
		"DATABASE_URL" + theme.KeyValueSeparator + viewmodel.MaskedValue,
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view não contém %q:\n%s", want, view)
		}
	}
	lines := strings.Split(view, "\n")
	if lines[0] != "envault" {
		t.Fatalf("cabeçalho = %q, esperado só envault sem contagem", lines[0])
	}
	assertNoSecrets(t, "view da lista", view)
	assertNoSecrets(t, "saída da lista", out)
}

func TestListFilterFlow(t *testing.T) {
	sess := start(t)
	sess.tm.Type("/stripe")
	sess.waitFor("/ stripe")
	sess.tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	h, out := sess.finish()

	if got := h.list.VisibleNames(); !slices.Equal(got, []string{"stripe-test"}) {
		t.Fatalf("visíveis = %v", got)
	}
	if h.list.Filtering() {
		t.Fatal("enter deveria sair do modo de filtro mantendo o filtro")
	}
	view := h.View()
	if strings.Contains(view, "postgres-local") || strings.Contains(view, "redis") {
		t.Fatalf("filtro não escondeu as demais envs:\n%s", view)
	}
	assertNoSecrets(t, "saída do filtro", out)
}

func TestListFilterEscClears(t *testing.T) {
	sess := start(t)
	sess.tm.Type("/redis")
	sess.waitFor("/ redis")
	sess.tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	h, _ := sess.finish()
	if h.list.HasFilter() || len(h.list.VisibleNames()) != 3 {
		t.Fatalf("esc deveria limpar o filtro, visíveis = %v", h.list.VisibleNames())
	}
}

func TestListMarkToggleShowsPositions(t *testing.T) {
	sess := start(t)
	sess.tm.Type(" j ")
	sess.waitFor("SELECTED_ENVS=postgres-local,redis")
	sess.tm.Type("k ")
	sess.waitFor("SELECTED_ENVS=redis")
	h, out := sess.finish()

	if !slices.Equal(h.list.Marked(), []string{"redis"}) {
		t.Fatalf("marcadas = %v, esperado [redis]", h.list.Marked())
	}
	view := h.View()
	if !strings.Contains(view, "[1] redis") || !strings.Contains(view, "SELECTED_ENVS=redis") {
		t.Fatalf("view não mostra a posição e a linha da seleção:\n%s", view)
	}
	assertNoSecrets(t, "saída da marcação", out)
}

func TestListSelectedEnvsLineExact(t *testing.T) {
	sess := start(t)
	h := harness{list: New().SetEnvs(seedEnvs(), "")}
	line := func(view string) string {
		for _, l := range strings.Split(view, "\n") {
			if strings.HasPrefix(l, "SELECTED_ENVS=") {
				return l
			}
		}
		return ""
	}
	if got := line(h.View()); got != "SELECTED_ENVS=" {
		t.Fatalf("linha sem seleção = %q", got)
	}
	sess.tm.Type(" ")
	sess.waitFor("SELECTED_ENVS=postgres-local")
	sess.tm.Type("j ")
	sess.waitFor("SELECTED_ENVS=postgres-local,redis")
	h, out := sess.finish()
	if got := line(h.View()); got != "SELECTED_ENVS=postgres-local,redis" {
		t.Fatalf("linha com várias envs = %q", got)
	}
	assertNoSecrets(t, "saída da seleção", out)
}

func TestListQuitIntent(t *testing.T) {
	sess := start(t)
	sess.tm.Type("q")
	final, ok := finalModel(t, sess.tm).(harness)
	if !ok || final.last != IntentQuit {
		t.Fatalf("q deveria pedir para sair, intent = %v", final.last)
	}
}

func TestListKeyIntents(t *testing.T) {
	tests := map[string]Intent{
		"?": IntentHelp,
		"c": IntentDuplicate,
		"r": IntentRename,
		"d": IntentDelete,
		"n": IntentNew,
		"e": IntentEdit,
		"i": IntentImport,
		"l": IntentCompose,
		"j": IntentNone,
		"x": IntentNone,
	}
	for k, want := range tests {
		l := New().SetEnvs(seedEnvs(), "")
		_, _, got := l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		if got != want {
			t.Errorf("tecla %q: intent = %v, esperado %v", k, got, want)
		}
	}
	_, _, got := New().SetEnvs(seedEnvs(), "").Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got != IntentOpen {
		t.Errorf("enter: intent = %v, esperado abrir", got)
	}
}

func TestListMarkedPrunedAfterReload(t *testing.T) {
	l := New().SetEnvs(seedEnvs(), "")
	l = l.toggleMark()
	l = l.move(1).toggleMark()
	l = l.SetEnvs(seedEnvs()[1:], "")
	if !slices.Equal(l.Marked(), []string{"redis"}) {
		t.Fatalf("marcadas = %v, esperado [redis]", l.Marked())
	}
	if env, ok := l.Focused(); !ok || env.Name != "redis" {
		t.Fatalf("foco = %v, esperado redis", env.Name)
	}
}

func TestListMarkedEnvsFollowSelectionOrder(t *testing.T) {
	l := New().SetEnvs(seedEnvs(), "").WithMarked([]string{"stripe-test", "sumiu", "postgres-local"})
	var names []string
	for _, env := range l.MarkedEnvs() {
		names = append(names, env.Name)
	}
	if !slices.Equal(names, []string{"stripe-test", "postgres-local"}) {
		t.Fatalf("marcadas = %v", names)
	}
	l = l.RenameMarked("stripe-test", "stripe")
	if !slices.Equal(l.Marked(), []string{"stripe", "postgres-local"}) {
		t.Fatalf("renomear = %v", l.Marked())
	}
}

func TestListEmptyVault(t *testing.T) {
	l := New().SetEnvs(nil, "")
	view := l.View(theme.DefaultStyles(), termWidth, termHeight)
	if !strings.Contains(view, "nenhuma env no cofre") {
		t.Fatalf("view vazia inesperada:\n%s", view)
	}
	if _, ok := l.Focused(); ok {
		t.Fatal("lista vazia não deveria ter foco")
	}
	if text, _ := l.selection.SelectedEnvsLine(); !strings.Contains(l.SelectionView(theme.DefaultStyles(), termWidth), text) {
		t.Fatal("linha vazia da seleção ausente")
	}
}

func TestListFailedAndLoading(t *testing.T) {
	st := theme.DefaultStyles()
	if view := New().View(st, termWidth, termHeight); !strings.Contains(view, "carregando...") {
		t.Fatalf("view antes da carga:\n%s", view)
	}
	if view := New().MarkFailed().View(st, termWidth, termHeight); !strings.Contains(view, "não foi possível ler o cofre") {
		t.Fatalf("view com falha:\n%s", view)
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
