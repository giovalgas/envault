package cli

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/shared/config"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

var rootCommandNames = []string{
	"init", "list", "show", "get", "set", "unset", "new", "edit", "import",
	"rename", "copy", "delete", "plan", "load", "selection", "exec", "shell", "shell-init", "skill",
	"key", "completion",
}

type fakeTUI struct {
	calls   int
	session TUISession
	err     error
}

func (f *fakeTUI) run(_ context.Context, session TUISession) error {
	f.calls++
	f.session = session
	return f.err
}

func (ta *testApp) withTUI(err error) *fakeTUI {
	fake := &fakeTUI{err: err}
	ta.App.Wire.TUI = fake.run
	return fake
}

func TestRootRegistersAllCommands(t *testing.T) {
	ta := newTestApp(t)
	var names []string
	for _, cmd := range commands(ta.App) {
		names = append(names, cmd.Name())
	}
	if strings.Join(names, ",") != strings.Join(rootCommandNames, ",") {
		t.Fatalf("comandos = %v", names)
	}
}

func TestRootHelpListsAllCommands(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.run("--help"); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	for _, name := range rootCommandNames {
		if !strings.Contains(ta.Out.String(), "\n  "+name+" ") {
			t.Errorf("help não lista %q:\n%s", name, ta.Out.String())
		}
	}
}

func TestRootWithoutArgsOpensTUIOnlyInTerminal(t *testing.T) {
	tests := []struct {
		name          string
		stdin, stdout bool
		wantTUI       bool
	}{
		{name: "dois TTY", stdin: true, stdout: true, wantTUI: true},
		{name: "stdout redirecionado", stdin: true, stdout: false},
		{name: "stdin redirecionado", stdin: false, stdout: true},
		{name: "sem TTY", stdin: false, stdout: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := newTestApp(t)
			fake := ta.withTUI(nil)
			ta.setTerminal(tt.stdin, tt.stdout)
			if code := ta.run(); code != ExitOK {
				t.Fatalf("code = %d stderr = %q", code, ta.Err.String())
			}
			if got := fake.calls == 1; got != tt.wantTUI {
				t.Fatalf("TUI aberta = %v, esperado %v", got, tt.wantTUI)
			}
			printedHelp := strings.Contains(ta.Out.String(), "Available Commands")
			if printedHelp == tt.wantTUI {
				t.Fatalf("help impresso = %v com TUI = %v: %q", printedHelp, tt.wantTUI, ta.Out.String())
			}
		})
	}
}

func TestRootWithArgsNeverOpensTUI(t *testing.T) {
	ta := newTestApp(t)
	fake := ta.withTUI(nil)
	ta.setTerminal(true, true)
	for _, args := range [][]string{{"--version"}, {"--help"}, {"completion", "bash"}} {
		if code := ta.run(args...); code != ExitOK {
			t.Fatalf("%v: code = %d", args, code)
		}
	}
	if fake.calls != 0 {
		t.Fatalf("TUI aberta %d vezes", fake.calls)
	}
}

func TestRootWithoutTUIWiringPrintsHelp(t *testing.T) {
	ta := newTestApp(t)
	ta.setTerminal(true, true)
	if code := ta.run(); code != ExitOK || !strings.Contains(ta.Out.String(), "Available Commands") {
		t.Fatalf("code = %d stdout = %q", code, ta.Out.String())
	}
}

func TestRootTUISessionSharesVault(t *testing.T) {
	ta := newTestApp(t)
	t.Setenv(config.EnvDebug, "1")
	ta.seed(t,
		vault.Env{Name: "a", Vars: []vault.Var{{Key: "X", Value: "1"}}},
		vault.Env{Name: "b", Vars: []vault.Var{{Key: "X", Value: "2"}}},
	)
	fake := ta.withTUI(nil)
	ta.setTerminal(true, true)
	if code := ta.run(); code != ExitOK {
		t.Fatalf("code = %d stderr = %q", code, ta.Err.String())
	}
	session := fake.session
	if !session.Debug || session.Stdin != ta.In || session.Stdout != ta.Out {
		t.Fatalf("sessão = %+v", session)
	}
	if session.Vault.BeginCreateEnv == nil || session.Vault.BeginEditEnv == nil || session.Vault.ImportEnv == nil {
		t.Fatalf("use cases do cofre ausentes: %+v", session.Vault)
	}
	result, err := session.Compose.PlanLoad.Execute(context.Background(), composeusecase.PlanLoadInput{Envs: []string{"a", "b"}, Target: ".env-inexistente"})
	if err != nil {
		t.Fatalf("PlanLoad: %v", err)
	}
	if !slices.Equal(result.Plan.Conflicts, []string{"X"}) || result.Plan.Vars[0].From != "b" {
		t.Fatalf("plano = %+v", result.Plan)
	}
}

func TestRootTUISessionCarriesShellExport(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	t.Setenv(composeusecase.ExportFileVar, "/tmp/envault.x/exports")
	t.Setenv(composeusecase.ExportShellVar, "fish")
	fake := ta.withTUI(nil)
	ta.setTerminal(true, true)
	if code := ta.run(); code != ExitOK {
		t.Fatalf("code = %d stderr = %q", code, ta.Err.String())
	}
	session := fake.session
	if session.Export != (ShellExport{File: "/tmp/envault.x/exports", Dialect: "fish"}) || !session.Export.Active() {
		t.Fatalf("export = %+v", session.Export)
	}
	if session.Stderr != ta.Err || session.Compose.LoadShellExports == nil {
		t.Fatalf("sessão = %+v", session)
	}
}

func TestRootTUIErrorsMapToExitCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "cancelado", err: context.Canceled, want: ExitCanceled},
		{name: "cofre", err: vault.ErrNotInitialized, want: ExitNotInitialized},
		{name: "genérico", err: errors.New("terminal indisponível"), want: ExitError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := newTestApp(t)
			ta.withTUI(tt.err)
			ta.setTerminal(true, true)
			if code := ta.run(); code != tt.want {
				t.Fatalf("code = %d, esperado %d", code, tt.want)
			}
		})
	}
}

func TestRootTUIConfigErrorSkipsTUI(t *testing.T) {
	ta := newTestApp(t)
	fake := ta.withTUI(nil)
	ta.setTerminal(true, true)
	ta.App.LoadConfig = func() (config.Config, error) { return config.Config{}, errors.New("boom") }
	if code := ta.run(); code != ExitError || fake.calls != 0 {
		t.Fatalf("code = %d calls = %d", code, fake.calls)
	}
	t.Setenv(config.EnvKey, "curta")
	ta.App.LoadConfig = config.FromOS
	if code := ta.run(); code != ExitValidation || fake.calls != 0 {
		t.Fatalf("chave inválida: code = %d calls = %d", code, fake.calls)
	}
}
