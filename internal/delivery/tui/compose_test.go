package tui

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

const stackReadyText = "2 chaves"

const (
	secretXFromA = "valor-xa"
	secretXFromB = "valor-xb"
	secretYFromB = "valor-yb"
)

var composeSecrets = []string{secretXFromA, secretXFromB, secretYFromB}

type stack struct {
	store   VaultStore
	deps    Deps
	dir     string
	exports string
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
			ShellExports:   composeusecase.NewLoadShellExports(composeusecase.NewRenderShell(source), files),
			ExportDialect:  composeusecase.ShellZsh,
			Dir:            dir,
		},
		dir: dir,
	}
}

func (s stack) withWrapper(t *testing.T) stack {
	t.Helper()
	s.exports = filepath.Join(t.TempDir(), "exports")
	s.deps.ExportFile = s.exports
	return s
}

func (s stack) start(t *testing.T) *session {
	t.Helper()
	opts := Options{
		Actions:   Actions(s.deps),
		Clipboard: func(string) error { return nil },
	}
	tm := teatest.NewTestModel(t, New(context.Background(), s.store, opts), teatest.WithInitialTermSize(termWidth, termHeight))
	sess := &session{t: t, tm: tm}
	sess.waitFor(stackReadyText)
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

func (s *session) assertSeen(texts ...string) {
	s.t.Helper()
	for _, text := range texts {
		if !strings.Contains(s.seen.String(), text) {
			s.t.Fatalf("tela não mostrou %q:\n%s", text, s.seen.String())
		}
	}
}

func openComposeAB(t *testing.T, s stack) *session {
	t.Helper()
	sess := s.start(t)
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
	data, err := os.ReadFile(filepath.Clean(path))
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
	sess := s.start(t)
	sess.typeText(" l")
	sess.waitFor("montar prévia")
	m, _ := sess.finish()
	if !m.statusErr || m.compose.planned {
		t.Fatalf("status = %q planned = %v", m.status, m.compose.planned)
	}
}

func TestComposeTerminalIsDefaultWithWrapper(t *testing.T) {
	s := newStack(t, composeEnvs()...).withWrapper(t)
	sess := openComposeAB(t, s)
	sess.assertSeen("destino: terminal atual", "t: gravar em arquivo")
	sess.press(tea.KeyEnter)
	m, out := sess.collect()

	if m.exported != "2 variáveis de a, b exportadas no terminal" {
		t.Fatalf("exported = %q", m.exported)
	}
	want := "export X='" + secretXFromB + "'\nexport Y='" + secretYFromB + "'\n"
	if got := readFile(t, s.exports); got != want {
		t.Fatalf("exports = %q", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(s.exports)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("exports perm %v err %v", info, err)
		}
	}
	if entries, err := os.ReadDir(s.dir); err != nil || len(entries) != 0 {
		t.Fatalf("montagem no terminal gravou arquivo: %v %v", entries, err)
	}
	assertNoSecrets(t, "saída da montagem no terminal", out, composeSecrets...)
	var report bytes.Buffer
	if err := reportExported(&report, m); err != nil || report.String() != "envault: 2 variáveis de a, b exportadas no terminal\n" {
		t.Fatalf("report = %q err %v", report.String(), err)
	}
	assertNoSecrets(t, "relato da exportação", report.String(), composeSecrets...)
}

func TestComposeTerminalReportsMissingTemplateKeys(t *testing.T) {
	s := newStack(t, composeEnvs()...).withWrapper(t)
	writeFile(t, filepath.Join(s.dir, templateFileName), "Y=\nSENTRY_DSN=\n")
	sess := openComposeAB(t, s)
	sess.press(tea.KeyEnter)
	m, _ := sess.collect()
	if m.exported != "2 variáveis de a, b exportadas no terminal; sem valor no template: SENTRY_DSN" {
		t.Fatalf("exported = %q", m.exported)
	}
	if got := readFile(t, s.exports); got != "export Y='"+secretYFromB+"'\nexport X='"+secretXFromB+"'\n" {
		t.Fatalf("exports = %q", got)
	}
}

func TestComposeToggleToFileWithWrapper(t *testing.T) {
	s := newStack(t, composeEnvs()...).withWrapper(t)
	sess := openComposeAB(t, s)
	sess.typeText("t")
	sess.waitForAll("destino: .env", "t: exportar no terminal")
	sess.press(tea.KeyEnter)
	sess.waitFor("gravado")
	m, out := sess.finish()

	if m.exported != "" || m.screen != screenList {
		t.Fatalf("exported %q tela %v", m.exported, m.screen)
	}
	if got := readFile(t, filepath.Join(s.dir, ".env")); !strings.Contains(got, "X="+secretXFromB) {
		t.Fatalf(".env = %q", got)
	}
	if _, err := os.Stat(s.exports); !os.IsNotExist(err) {
		t.Fatalf("exports não deveria existir: %v", err)
	}
	assertNoSecrets(t, "saída da montagem em arquivo", out, composeSecrets...)
}

func TestComposeTargetPaneTogglesBack(t *testing.T) {
	s := newStack(t, composeEnvs()...).withWrapper(t)
	sess := openComposeAB(t, s)
	sess.press(tea.KeyShiftTab)
	sess.typeText("t")
	sess.waitFor("destino: .env")
	sess.typeText("x")
	sess.press(tea.KeyTab)
	sess.typeText("t")
	sess.waitFor("destino: terminal atual")
	m, _ := sess.finish()
	if m.compose.dest != destTerminal || m.compose.targetValue() != ".envx" || m.compose.target.Focused() {
		t.Fatalf("dest %v alvo %q foco %v", m.compose.dest, m.compose.targetValue(), m.compose.target.Focused())
	}
}

func TestComposeWithoutWrapperOffersOnlyFile(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	sess := openComposeAB(t, s)
	sess.assertSeen("destino: .env", "sem wrapper: só arquivo", `eval "$(envault shell-init zsh)"`)
	sess.typeText("t")
	sess.waitFor("sem o wrapper de shell")
	m, _ := sess.finish()
	if m.compose.dest != destFile || !m.statusWarn || !strings.Contains(m.status, `eval "$(envault shell-init zsh)"`) {
		t.Fatalf("dest %v status %q warn %v", m.compose.dest, m.status, m.statusWarn)
	}
	if m.exported != "" {
		t.Fatalf("exported = %q", m.exported)
	}
}

func TestComposeTerminalExportErrorKeepsCompose(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	s.deps.ExportFile = t.TempDir()
	sess := openComposeAB(t, s)
	sess.press(tea.KeyEnter)
	sess.waitFor("exportar no terminal")
	m, out := sess.finish()
	if m.screen != screenCompose || !m.statusErr || m.exported != "" {
		t.Fatalf("tela %v status %q", m.screen, m.status)
	}
	assertNoSecrets(t, "saída do erro de exportação", out, composeSecrets...)
}
