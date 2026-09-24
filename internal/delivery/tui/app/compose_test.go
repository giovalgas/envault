package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/giovalgas/envault/internal/compose/infra/envfile"
	"github.com/giovalgas/envault/internal/compose/infra/gitignore"
	"github.com/giovalgas/envault/internal/compose/infra/selectionfile"
	"github.com/giovalgas/envault/internal/compose/infra/templatefile"
	"github.com/giovalgas/envault/internal/compose/infra/vaultsource"
	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/tui/viewmodel"
	"github.com/giovalgas/envault/internal/shared/config"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/editor"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	"github.com/giovalgas/envault/internal/vault/infra/sourcefile"
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
	source    *vaultsource.Source
	repo      *encryptedfile.Repository
	store     VaultStore
	deps      Deps
	selection SelectionUseCases
	dir       string
	vaultDir  string
	exports   string
	clipboard *clipboardSpy
}

type clipboardSpy struct {
	copied []string
	err    error
}

func (c *clipboardSpy) write(text string) error {
	if c.err != nil {
		return c.err
	}
	c.copied = append(c.copied, text)
	return nil
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
	vaultDir := t.TempDir()
	repo := encryptedfile.New(config.PathsIn(vaultDir), keys)
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
	selections := selectionfile.New(func() (string, error) { return vaultDir, nil })
	dir := t.TempDir()
	return stack{
		source: source,
		repo:   repo,
		selection: SelectionUseCases{
			GetSelection:  composeusecase.NewGetSelection(source, selections),
			SaveSelection: composeusecase.NewSaveSelection(selections, nil),
		},
		vaultDir:  vaultDir,
		clipboard: &clipboardSpy{},
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
			ImportEnv:      vaultusecase.NewImportEnv(repo, sourcefile.New(), nil),
			LoadTemplate:   composeusecase.NewLoadTemplate(templatefile.New()),
			PlanLoad:       composeusecase.NewPlanLoad(source, files, ignore),
			LoadEnvFile:    composeusecase.NewLoadEnvFile(source, files, ignore),
			ShellExports:   composeusecase.NewLoadShellExports(composeusecase.NewRenderShell(source), files),
			RenderEnvFile:  composeusecase.NewRenderEnvFile(source),
			ExportDialect:  composeusecase.ShellZsh,
			Dir:            dir,
		},
		dir: dir,
	}
}

func (s stack) options() Options {
	return Options{
		Actions:   Actions(s.deps),
		Clipboard: s.clipboard.write,
		Selection: s.selection,
	}
}

func (s stack) withWrapper(t *testing.T) stack {
	t.Helper()
	s.exports = filepath.Join(t.TempDir(), "exports")
	s.deps.ExportFile = s.exports
	return s
}

type fixedGitignore composeusecase.GitignoreStatus

func (f fixedGitignore) IsIgnored(string) composeusecase.GitignoreStatus {
	return composeusecase.GitignoreStatus(f)
}

func (s stack) withGitignore(checker composeusecase.GitignoreChecker) stack {
	files := envfile.New()
	s.deps.PlanLoad = composeusecase.NewPlanLoad(s.source, files, checker)
	s.deps.LoadEnvFile = composeusecase.NewLoadEnvFile(s.source, files, checker)
	return s
}

func (s stack) start(t *testing.T) *session {
	t.Helper()
	tm := teatest.NewTestModel(t, New(context.Background(), s.store, s.options()), teatest.WithInitialTermSize(termWidth, termHeight))
	sess := &session{t: t, tm: tm}
	sess.waitFor(stackReadyText)
	return sess
}

func (s stack) get(t *testing.T, name string) vault.Env {
	t.Helper()
	snapshot, err := s.repo.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	env, err := snapshot.Get(name)
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
	sess.waitFor("SELECTED_ENVS=a,b")
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
	s := newStack(t, composeEnvs()...).withGitignore(fixedGitignore(composeusecase.GitignoreNotIgnored))
	sess := openComposeAB(t, s)
	sess.press(tea.KeyShiftTab)
	for range len(viewmodel.DefaultEnvFile) {
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

func TestComposeBackReturnsToList(t *testing.T) {
	for _, k := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyCtrlC}} {
		t.Run(k.String(), func(t *testing.T) {
			s := newStack(t, composeEnvs()...)
			sess := openComposeAB(t, s)
			sess.tm.Send(k)
			sess.waitFor("2 chaves")
			m, _ := sess.finish()
			if m.screen != screenList || len(m.list.Marked()) != 2 {
				t.Fatalf("tela = %v marcadas = %v", m.screen, m.list.Marked())
			}
		})
	}
}

func TestComposeTemplateParseErrorIsShown(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	writeFile(t, filepath.Join(s.dir, composeusecase.DefaultTemplateFile), "1BAD=\n")
	sess := s.start(t)
	sess.typeText(" l")
	sess.waitFor("montar prévia")
	m, _ := sess.finish()
	if !m.statusErr || m.compose.Planned() {
		t.Fatalf("status = %q planned = %v", m.status, m.compose.Planned())
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
	writeFile(t, filepath.Join(s.dir, composeusecase.DefaultTemplateFile), "Y=\nSENTRY_DSN=\n")
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

func TestComposeCopiesEnvFileToClipboard(t *testing.T) {
	s := newStack(t, composeEnvs()...).withWrapper(t)
	writeFile(t, filepath.Join(s.dir, composeusecase.DefaultTemplateFile), "Y=\nX=\nSENTRY_DSN=\n")
	sess := openComposeAB(t, s)
	sess.assertSeen("y: copiar .env para o clipboard")
	sess.typeText("y")
	sess.waitFor("copiado para o clipboard")
	m, out := sess.finish()

	if m.screen != screenList || m.exported != "" {
		t.Fatalf("tela %v exported %q", m.screen, m.exported)
	}
	if !m.statusWarn || !strings.Contains(m.status, ".env com 2 variáveis de a, b copiado para o clipboard") || !strings.Contains(m.status, "sem valor no template: SENTRY_DSN") {
		t.Fatalf("status = %q", m.status)
	}
	want := "Y=" + secretYFromB + "\nX=" + secretXFromB + "\n"
	if len(s.clipboard.copied) != 1 || s.clipboard.copied[0] != want {
		t.Fatalf("clipboard = %q", s.clipboard.copied)
	}
	if entries, err := os.ReadDir(s.dir); err != nil || len(entries) != 1 {
		t.Fatalf("clipboard gravou arquivo: %v %v", entries, err)
	}
	if _, err := os.Stat(s.exports); !os.IsNotExist(err) {
		t.Fatalf("exports não deveria existir: %v", err)
	}
	assertNoSecrets(t, "saída da cópia para o clipboard", out, composeSecrets...)
}

func TestComposeClipboardErrorKeepsCompose(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	s.clipboard.err = errors.New("sem xclip")
	sess := openComposeAB(t, s)
	sess.typeText("y")
	sess.waitFor("copiar para o clipboard: sem xclip")
	m, out := sess.finish()
	if m.screen != screenCompose || !m.statusErr {
		t.Fatalf("tela %v status %q", m.screen, m.status)
	}
	if entries, err := os.ReadDir(s.dir); err != nil || len(entries) != 0 {
		t.Fatalf("erro de clipboard gravou arquivo: %v %v", entries, err)
	}
	assertNoSecrets(t, "saída do erro de clipboard", out, composeSecrets...)
}
