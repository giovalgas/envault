package tui

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/giovalgas/envault/internal/compose/infra/envfile"
	"github.com/giovalgas/envault/internal/compose/infra/gitignore"
	"github.com/giovalgas/envault/internal/compose/infra/vaultsource"
	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/shared/config"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/editor"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	secretXFromA = "valor-x-de-a-7f3c"
	secretXFromB = "valor-x-de-b-91d2"
	secretYFromB = "valor-y-de-b-44aa"
)

var composeSecrets = []string{secretXFromA, secretXFromB, secretYFromB}

type stack struct {
	store VaultStore
	deps  Deps
	dir   string
}

func composeEnvs() []vault.Env {
	return []vault.Env{
		{Name: "a", Vars: []vault.Var{{Key: "X", Value: secretXFromA}}},
		{Name: "b", Vars: []vault.Var{{Key: "X", Value: secretXFromB}, {Key: "Y", Value: secretYFromB}}},
	}
}

func newStack(t *testing.T, envs ...vault.Env) stack {
	t.Helper()
	key, err := vault.NewKey()
	if err != nil {
		t.Fatalf("gerar chave: %v", err)
	}
	keys, err := keystore.NewStatic(key)
	if err != nil {
		t.Fatalf("criar keystore: %v", err)
	}
	repo := encryptedfile.New(config.PathsIn(t.TempDir()), keys)
	ctx := context.Background()
	if _, err := repo.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, env := range envs {
		if err := repo.Update(ctx, func(s *vault.Snapshot) error {
			_, err := s.Create(env, time.Now())
			return err
		}); err != nil {
			t.Fatalf("criar %s: %v", env.Name, err)
		}
	}
	sessions := editor.NewInteractive(editor.Options{})
	list := vaultusecase.NewListEnvs(repo)
	source := vaultsource.New(func() (*vaultusecase.ListEnvs, error) { return list, nil })
	files, ignore := envfile.New(), gitignore.New()
	dir := t.TempDir()
	return stack{
		store: VaultStore{
			ListEnvs:  list,
			ShowEnv:   vaultusecase.NewShowEnv(repo),
			CopyEnv:   vaultusecase.NewCopyEnv(repo, nil),
			RenameEnv: vaultusecase.NewRenameEnv(repo, nil),
			DeleteEnv: vaultusecase.NewDeleteEnv(repo),
		},
		deps: Deps{
			BeginCreateEnv: vaultusecase.NewBeginCreateEnv(repo, sessions, nil),
			BeginEditEnv:   vaultusecase.NewBeginEditEnv(repo, sessions, nil),
			ImportEnv:      vaultusecase.NewImportEnv(repo, nil),
			PlanLoad:       composeusecase.NewPlanLoad(source, files, ignore),
			LoadEnvFile:    composeusecase.NewLoadEnvFile(source, files, ignore),
			Dir:            dir,
		},
		dir: dir,
	}
}

func (s stack) start(t *testing.T, first string) *session {
	t.Helper()
	opts := Options{
		Actions:   Actions(s.deps),
		Clipboard: func(string) error { return nil },
	}
	tm := teatest.NewTestModel(t, New(context.Background(), s.store, opts), teatest.WithInitialTermSize(termWidth, termHeight))
	sess := &session{t: t, tm: tm}
	sess.waitFor(first)
	return sess
}

func (s stack) get(t *testing.T, name string) vault.Env {
	t.Helper()
	env, err := s.store.Get(context.Background(), name)
	if err != nil {
		t.Fatalf("Get(%s): %v", name, err)
	}
	return env
}

func (s *session) waitForAll(texts ...string) {
	s.t.Helper()
	var seen bytes.Buffer
	teatest.WaitFor(s.t, s.tm.Output(), func(b []byte) bool {
		seen.Write(b)
		for _, text := range texts {
			if !bytes.Contains(seen.Bytes(), []byte(text)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(waitTimeout), teatest.WithCheckInterval(10*time.Millisecond))
	s.seen.Write(seen.Bytes())
}

func openComposeAB(t *testing.T, s stack) *session {
	t.Helper()
	sess := s.start(t, "2 chaves")
	sess.typeText(" j ")
	sess.waitFor("2 marcadas")
	sess.typeText("l")
	sess.waitFor("prévia: a, b")
	return sess
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

func resolvedKey(t *testing.T, m Model, key string) (from string, shadows []string) {
	t.Helper()
	for _, resolved := range m.compose.plan.Vars {
		if resolved.Key == key {
			return resolved.From, resolved.ShadowNames()
		}
	}
	t.Fatalf("chave %s ausente da prévia: %+v", key, m.compose.plan.Keys())
	return "", nil
}

func TestComposeOrderDecidesWinner(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	sess := openComposeAB(t, s)
	sess.typeText("J")
	sess.waitFor("prévia: b, a")
	sess.press(tea.KeyTab)
	sess.typeText(" ")
	sess.waitFor("sombreada em b")
	m, out := sess.finish()

	if m.screen != screenCompose {
		t.Fatalf("tela = %v, esperado montagem", m.screen)
	}
	if strings.Join(m.compose.order, ",") != "b,a" {
		t.Fatalf("ordem = %v", m.compose.order)
	}
	from, shadows := resolvedKey(t, m, "X")
	if from != "a" || strings.Join(shadows, ",") != "b" || !slices.Contains(m.compose.plan.Conflicts, "X") {
		t.Fatalf("X de %q sombras %v conflitos %v", from, shadows, m.compose.plan.Conflicts)
	}
	view := m.View()
	for _, want := range []string{"1. b", "2. a", "conflito -1", "sombreada em b", "a de baixo vence"} {
		if !strings.Contains(view, want) {
			t.Errorf("view não contém %q:\n%s", want, view)
		}
	}
	assertNoSecrets(t, "view da montagem", view, composeSecrets...)
	assertNoSecrets(t, "saída da montagem", out, composeSecrets...)
}

func TestComposeInitialOrderIsMarkingOrder(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	sess := openComposeAB(t, s)
	sess.typeText("K")
	m, _ := sess.finish()
	from, shadows := resolvedKey(t, m, "X")
	if from != "b" || strings.Join(shadows, ",") != "a" {
		t.Fatalf("X de %q sombras %v", from, shadows)
	}
	if strings.Join(m.compose.order, ",") != "a,b" {
		t.Fatalf("K no topo não deveria mover: %v", m.compose.order)
	}
}

func TestComposeExistingTargetOffersThreeChoices(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	target := filepath.Join(s.dir, ".env")
	writeFile(t, target, "X=antigo\nLOCAL=1\n")
	sess := openComposeAB(t, s)
	sess.press(tea.KeyEnter)
	sess.waitForAll("Destino já existe", "sobrescrever", "mesclar", "cancelar")
	if got := readFile(t, target); got != "X=antigo\nLOCAL=1\n" {
		t.Fatalf("destino alterado antes da escolha: %q", got)
	}
	sess.press(tea.KeyTab)
	sess.press(tea.KeyEnter)
	sess.waitFor("mesclado")
	m, out := sess.finish()

	if m.screen != screenList || m.modal != nil {
		t.Fatalf("tela = %v modal = %v", m.screen, m.modal)
	}
	want := "X=" + secretXFromB + "\nLOCAL=1\nY=" + secretYFromB + "\n"
	if got := readFile(t, target); got != want {
		t.Fatalf("destino = %q, esperado %q", got, want)
	}
	assertNoSecrets(t, "saída da montagem", out, composeSecrets...)
}

func TestComposeExistingTargetChoices(t *testing.T) {
	tests := []struct {
		name  string
		tabs  int
		want  string
		label string
	}{
		{name: "sobrescrever", tabs: 0, want: "X=" + secretXFromB + "\nY=" + secretYFromB + "\n", label: "sobrescrito"},
		{name: "cancelar", tabs: 2, want: "LOCAL=1\n", label: "prévia: a, b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStack(t, composeEnvs()...)
			target := filepath.Join(s.dir, ".env")
			writeFile(t, target, "LOCAL=1\n")
			sess := openComposeAB(t, s)
			sess.press(tea.KeyEnter)
			sess.waitFor("Destino já existe")
			for range tt.tabs {
				sess.press(tea.KeyTab)
			}
			sess.press(tea.KeyEnter)
			if tt.name != "cancelar" {
				sess.waitFor(tt.label)
			}
			m, _ := sess.finish()
			if got := readFile(t, target); got != tt.want {
				t.Fatalf("destino = %q, esperado %q", got, tt.want)
			}
			if tt.name == "cancelar" && (m.screen != screenCompose || m.modal != nil) {
				t.Fatalf("cancelar deveria voltar à montagem: tela %v modal %v", m.screen, m.modal)
			}
		})
	}
}

func TestComposeEditableTargetAndGitignoreWarning(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git não encontrado no PATH")
	}
	s := newStack(t, composeEnvs()...)
	if out, err := exec.Command(git, "init", "-q", s.dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	sess := openComposeAB(t, s)
	sess.press(tea.KeyShiftTab)
	for range len(defaultEnvFile) {
		sess.press(tea.KeyBackspace)
	}
	sess.typeText("app.env")
	sess.press(tea.KeyEnter)
	sess.waitFor(".gitignore")
	m, _ := sess.finish()

	if !m.statusWarn || !strings.Contains(m.status, "app.env gravado com 2 variáveis de a, b") || !strings.Contains(m.status, "app.env não está coberto pelo .gitignore") {
		t.Fatalf("status = %q warn %v", m.status, m.statusWarn)
	}
	if _, err := os.Stat(filepath.Join(s.dir, ".env")); !os.IsNotExist(err) {
		t.Fatalf(".env não deveria existir: %v", err)
	}
	if got := readFile(t, filepath.Join(s.dir, "app.env")); !strings.Contains(got, "Y="+secretYFromB) {
		t.Fatalf("app.env = %q", got)
	}
	if _, err := os.Stat(filepath.Join(s.dir, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf(".gitignore não deveria ser criado: %v", err)
	}
}

func TestComposeTemplateChecklist(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	writeFile(t, filepath.Join(s.dir, templateFileName), "Y=\nPORT=3000\nSENTRY_DSN=\n")
	sess := openComposeAB(t, s)
	m, out := sess.finish()

	if !m.compose.tmpl.found || strings.Join(m.compose.plan.Missing, ",") != "SENTRY_DSN" || strings.Join(m.compose.plan.Extra, ",") != "X" {
		t.Fatalf("plano = %+v", m.compose.plan)
	}
	if strings.Join(m.compose.plan.Keys(), ",") != "Y,PORT,X" {
		t.Fatalf("ordem = %v", m.compose.plan.Keys())
	}
	view := m.View()
	for _, want := range []string{"template .env.example: 2 de 3 preenchidas", "PORT (padrão)", "padrão do template", "fora do template: X", "SENTRY_DSN faltando"} {
		if !strings.Contains(view, want) {
			t.Errorf("view não contém %q:\n%s", want, view)
		}
	}
	assertNoSecrets(t, "view com template", view, append(composeSecrets, "3000")...)
	assertNoSecrets(t, "saída com template", out, composeSecrets...)
}

func TestComposeBackReturnsToList(t *testing.T) {
	for _, k := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyCtrlC}} {
		t.Run(k.String(), func(t *testing.T) {
			s := newStack(t, composeEnvs()...)
			sess := openComposeAB(t, s)
			sess.tm.Send(k)
			sess.waitFor("2 chaves")
			m, _ := sess.finish()
			if m.screen != screenList || len(m.list.marked) != 2 {
				t.Fatalf("tela = %v marcadas = %v", m.screen, m.list.marked)
			}
		})
	}
}

func TestComposeRejectsEmptyTarget(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	sess := openComposeAB(t, s)
	sess.press(tea.KeyShiftTab)
	for range len(defaultEnvFile) {
		sess.press(tea.KeyBackspace)
	}
	sess.press(tea.KeyEnter)
	sess.waitFor("informe o arquivo de destino")
	m, _ := sess.finish()
	if !m.statusErr || m.screen != screenCompose {
		t.Fatalf("status = %q tela = %v", m.status, m.screen)
	}
}

func TestComposeTemplateParseErrorIsShown(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	writeFile(t, filepath.Join(s.dir, templateFileName), "1BAD=\n")
	sess := s.start(t, "2 chaves")
	sess.typeText(" l")
	sess.waitFor("montar prévia")
	m, _ := sess.finish()
	if !m.statusErr || m.compose.planned {
		t.Fatalf("status = %q planned = %v", m.status, m.compose.planned)
	}
}
