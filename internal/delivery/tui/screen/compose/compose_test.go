package compose

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/giovalgas/envault/internal/compose/infra/envfile"
	"github.com/giovalgas/envault/internal/compose/infra/gitignore"
	"github.com/giovalgas/envault/internal/compose/infra/templatefile"
	"github.com/giovalgas/envault/internal/compose/infra/vaultsource"
	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/tui/theme"
	"github.com/giovalgas/envault/internal/delivery/tui/viewmodel"
	"github.com/giovalgas/envault/internal/shared/config"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	waitTimeout = 5 * time.Second
	termWidth   = 120
	termHeight  = 32

	secretXFromA = "valor-xa"
	secretXFromB = "valor-xb"
	secretYFromB = "valor-yb"
)

var composeSecrets = []string{secretXFromA, secretXFromB, secretYFromB}

func newDeps(t *testing.T) Deps {
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
	envs := []vault.Env{
		{Name: "a", Vars: []vault.Var{{Key: "X", Value: secretXFromA}}},
		{Name: "b", Vars: []vault.Var{{Key: "X", Value: secretXFromB}, {Key: "Y", Value: secretYFromB}}},
	}
	for _, env := range envs {
		if err := repo.Update(ctx, func(s *vault.Snapshot) error {
			_, err := s.Create(env, time.Now())
			return err
		}); err != nil {
			t.Fatalf("criar %s: %v", env.Name, err)
		}
	}
	list := vaultusecase.NewListEnvs(repo)
	source := vaultsource.New(func() (*vaultusecase.ListEnvs, error) { return list, nil })
	files, ignore := envfile.New(), gitignore.New()
	return Deps{
		Template: composeusecase.NewLoadTemplate(templatefile.New()),
		Plan:     composeusecase.NewPlanLoad(source, files, ignore),
		Load:     composeusecase.NewLoadEnvFile(source, files, ignore),
		Export:   composeusecase.NewLoadShellExports(composeusecase.NewRenderShell(source), files),
		Dialect:  composeusecase.ShellZsh,
		Dir:      t.TempDir(),
	}
}

func withWrapper(t *testing.T, deps Deps) Deps {
	t.Helper()
	deps.ExportFile = filepath.Join(t.TempDir(), "exports")
	return deps
}

type harness struct {
	compose Model
	effect  Effect
	planErr error
}

func (h harness) Init() tea.Cmd {
	return nil
}

func (h harness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case PlanMsg:
		updated, current := h.compose.OnPlan(msg)
		h.compose = updated
		if current {
			h.planErr = msg.Err
		}
	case tea.KeyMsg:
		updated, cmd, effect := h.compose.Update(msg)
		h.compose = updated
		if effect.Intent != IntentNone {
			h.effect = effect
		}
		return h, cmd
	}
	return h, nil
}

func (h harness) View() string {
	status := h.effect.Status + h.effect.Warning
	if h.effect.Err != nil {
		status = h.effect.Err.Error()
	}
	return h.compose.View(theme.DefaultStyles(), termWidth, termHeight-1) + "\n" + status
}

type session struct {
	t    *testing.T
	tm   *teatest.TestModel
	seen bytes.Buffer
}

func open(t *testing.T, deps Deps) *session {
	t.Helper()
	c, cmd := New(context.Background(), []string{"a", "b"}, deps).PlanCmd()
	tm := teatest.NewTestModel(t, harness{compose: c}, teatest.WithInitialTermSize(termWidth, termHeight))
	tm.Send(cmd())
	sess := &session{t: t, tm: tm}
	sess.waitFor("prévia: a, b")
	return sess
}

func (s *session) waitFor(text string) {
	s.t.Helper()
	var last []byte
	teatest.WaitFor(s.t, s.tm.Output(), func(b []byte) bool {
		last = b
		return bytes.Contains(b, []byte(text))
	}, teatest.WithDuration(waitTimeout), teatest.WithCheckInterval(10*time.Millisecond))
	s.seen.Write(last)
}

func (s *session) typeText(text string) {
	s.tm.Type(text)
}

func (s *session) press(k tea.KeyType) {
	s.tm.Send(tea.KeyMsg{Type: k})
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
	rest, err := io.ReadAll(s.tm.FinalOutput(s.t, teatest.WithFinalTimeout(waitTimeout)))
	if err != nil {
		s.t.Fatalf("ler saída: %v", err)
	}
	s.seen.Write(rest)
	return final, s.seen.String()
}

func assertNoSecrets(t *testing.T, where, text string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Fatalf("%s contém o valor %q", where, secret)
		}
	}
}

func resolvedKey(t *testing.T, c Model, key string) (from string, shadows []string) {
	t.Helper()
	for _, resolved := range c.Plan().Vars {
		if resolved.Key == key {
			return resolved.From, resolved.Shadows
		}
	}
	t.Fatalf("chave %s ausente da prévia: %+v", key, c.Plan().Keys())
	return "", nil
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestComposeOrderDecidesWinner(t *testing.T) {
	sess := open(t, newDeps(t))
	sess.typeText("J")
	sess.waitFor("prévia: b, a")
	sess.press(tea.KeyTab)
	sess.typeText(" ")
	sess.waitFor("sombreada em b")
	h, out := sess.finish()

	if h.effect.Intent != IntentReordered {
		t.Fatalf("J deveria reordenar, efeito %+v", h.effect)
	}
	if strings.Join(h.compose.Order(), ",") != "b,a" {
		t.Fatalf("ordem = %v", h.compose.Order())
	}
	from, shadows := resolvedKey(t, h.compose, "X")
	if from != "a" || strings.Join(shadows, ",") != "b" || !slices.Contains(h.compose.Plan().Conflicts, "X") {
		t.Fatalf("X de %q sombras %v conflitos %v", from, shadows, h.compose.Plan().Conflicts)
	}
	view := h.View()
	for _, want := range []string{"1. b", "2. a", "conflito -1", "sombreada em b", "a de baixo vence"} {
		if !strings.Contains(view, want) {
			t.Errorf("view não contém %q:\n%s", want, view)
		}
	}
	assertNoSecrets(t, "view da montagem", view, composeSecrets...)
	assertNoSecrets(t, "saída da montagem", out, composeSecrets...)
}

func TestComposeInitialOrderIsMarkingOrder(t *testing.T) {
	sess := open(t, newDeps(t))
	sess.typeText("K")
	h, _ := sess.finish()
	from, shadows := resolvedKey(t, h.compose, "X")
	if from != "b" || strings.Join(shadows, ",") != "a" {
		t.Fatalf("X de %q sombras %v", from, shadows)
	}
	if strings.Join(h.compose.Order(), ",") != "a,b" || h.effect.Intent == IntentReordered {
		t.Fatalf("K no topo não deveria mover: %v %+v", h.compose.Order(), h.effect)
	}
	if !strings.Contains(h.View(), "conflito +1") {
		t.Fatalf("conflito recolhido ausente:\n%s", h.View())
	}
}

func TestComposeTemplateChecklist(t *testing.T) {
	deps := newDeps(t)
	writeFile(t, filepath.Join(deps.Dir, composeusecase.DefaultTemplateFile), "Y=\nPORT=3000\nSENTRY_DSN=\n")
	sess := open(t, deps)
	h, out := sess.finish()

	plan := h.compose.Plan()
	if !h.compose.Template().Found || strings.Join(plan.Missing, ",") != "SENTRY_DSN" || strings.Join(plan.Extra, ",") != "X" {
		t.Fatalf("plano = %+v", plan)
	}
	if strings.Join(plan.Keys(), ",") != "Y,PORT,X" {
		t.Fatalf("ordem = %v", plan.Keys())
	}
	view := h.View()
	for _, want := range []string{"template .env.example: 2 de 3 preenchidas", "PORT (padrão)", "padrão do template", "fora do template: X", "SENTRY_DSN faltando"} {
		if !strings.Contains(view, want) {
			t.Errorf("view não contém %q:\n%s", want, view)
		}
	}
	assertNoSecrets(t, "view com template", view, append(composeSecrets, "3000")...)
	assertNoSecrets(t, "saída com template", out, composeSecrets...)
}

func TestComposeTemplateParseErrorComesInPlan(t *testing.T) {
	deps := newDeps(t)
	writeFile(t, filepath.Join(deps.Dir, composeusecase.DefaultTemplateFile), "1BAD=\n")
	c, cmd := New(context.Background(), []string{"a"}, deps).PlanCmd()
	msg, ok := cmd().(PlanMsg)
	if !ok || msg.Err == nil || !strings.Contains(msg.Err.Error(), composeusecase.DefaultTemplateFile) {
		t.Fatalf("plano = %+v", msg)
	}
	c, current := c.OnPlan(msg)
	if !current || c.Planned() {
		t.Fatalf("current %v planned %v", current, c.Planned())
	}
}

func TestComposeStalePlanIsIgnored(t *testing.T) {
	c, first := New(context.Background(), []string{"a", "b"}, newDeps(t)).PlanCmd()
	c, second := c.PlanCmd()
	c, current := c.OnPlan(first().(PlanMsg))
	if current || c.Planned() {
		t.Fatal("prévia antiga não deveria ser aplicada")
	}
	if _, current = c.OnPlan(second().(PlanMsg)); !current {
		t.Fatal("prévia atual deveria ser aplicada")
	}
}

func TestComposeTerminalIsDefaultWithWrapper(t *testing.T) {
	sess := open(t, withWrapper(t, newDeps(t)))
	h, _ := sess.finish()
	view := h.View()
	for _, want := range []string{"destino: terminal atual", "t: gravar em arquivo"} {
		if !strings.Contains(view, want) {
			t.Errorf("view não contém %q:\n%s", want, view)
		}
	}
	_, cmd, effect := h.compose.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || effect.Status != "exportando..." {
		t.Fatalf("enter deveria exportar, efeito %+v", effect)
	}
	exported, ok := cmd().(ExportedMsg)
	if !ok || exported.Err != nil || viewmodel.ExportedText(exported.Result) != "2 variáveis de a, b exportadas no terminal" {
		t.Fatalf("exportação = %+v", exported)
	}
}

func TestComposeTargetPaneTogglesBack(t *testing.T) {
	sess := open(t, withWrapper(t, newDeps(t)))
	sess.press(tea.KeyShiftTab)
	sess.typeText("t")
	sess.waitFor("destino: .env")
	sess.typeText("x")
	sess.press(tea.KeyTab)
	sess.typeText("t")
	sess.waitFor("destino: terminal atual")
	h, _ := sess.finish()
	c := h.compose
	if c.dest != destTerminal || c.targetValue() != ".envx" || c.target.Focused() {
		t.Fatalf("dest %v alvo %q foco %v", c.dest, c.targetValue(), c.target.Focused())
	}
}

func TestComposeWithoutWrapperOffersOnlyFile(t *testing.T) {
	sess := open(t, newDeps(t))
	sess.typeText("t")
	sess.waitFor("sem o wrapper de shell")
	h, out := sess.finish()
	for _, want := range []string{"destino: .env", "sem wrapper: só arquivo", `eval "$(envault shell-init zsh)"`} {
		if !strings.Contains(out, want) {
			t.Errorf("tela não mostrou %q", want)
		}
	}
	if h.compose.dest != destFile || h.effect.Intent != IntentStatus || !strings.Contains(h.effect.Warning, `eval "$(envault shell-init zsh)"`) {
		t.Fatalf("dest %v efeito %+v", h.compose.dest, h.effect)
	}
}

func TestComposeRejectsEmptyTarget(t *testing.T) {
	sess := open(t, newDeps(t))
	sess.press(tea.KeyShiftTab)
	for range len(viewmodel.DefaultEnvFile) {
		sess.press(tea.KeyBackspace)
	}
	sess.press(tea.KeyEnter)
	sess.waitFor("informe o arquivo de destino")
	h, _ := sess.finish()
	if h.effect.Err == nil {
		t.Fatalf("efeito = %+v", h.effect)
	}
}

func TestComposeWriteBeforePlanIsRefused(t *testing.T) {
	c := New(context.Background(), []string{"a"}, newDeps(t))
	if _, err := c.WriteCmd(composeusecase.RefuseExisting); err == nil {
		t.Fatal("gravar sem prévia deveria falhar")
	}
	_, cmd, effect := c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || effect.Err == nil {
		t.Fatalf("enter sem prévia: efeito %+v", effect)
	}
}

func TestComposeCloseAndHelpIntents(t *testing.T) {
	c := New(context.Background(), []string{"a"}, newDeps(t))
	for _, k := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyCtrlC}} {
		if _, _, effect := c.Update(k); effect.Intent != IntentClose {
			t.Errorf("%s: efeito %+v", k, effect)
		}
	}
	if _, _, effect := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}); effect.Intent != IntentHelp {
		t.Errorf("?: efeito %+v", effect)
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
