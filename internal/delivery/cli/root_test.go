package cli

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/compose/infra/envfile"
	"github.com/giovalgas/envault/internal/compose/infra/gitignore"
	"github.com/giovalgas/envault/internal/compose/infra/selectionfile"
	"github.com/giovalgas/envault/internal/compose/infra/templatefile"
	"github.com/giovalgas/envault/internal/compose/infra/vaultsource"
	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/clitest"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	"github.com/giovalgas/envault/internal/shared/config"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	"github.com/giovalgas/envault/internal/vault/infra/sourcefile"
)

type testApp struct{ *clitest.Harness }

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	return &testApp{Harness: clitest.New(t, Execute, testWiring)}
}

func testWiring(*clitest.Harness) app.Wiring {
	return app.Wiring{Vault: testVault, Compose: testCompose}
}

func testVault(cfg config.Config, _ app.Streams) (app.VaultUseCases, error) {
	keys, err := keystore.WithOverride(cfg.KeyOverride, keystore.NewFile(cfg.Paths.Key))
	if err != nil {
		return app.VaultUseCases{}, err
	}
	return app.NewVaultUseCases(app.VaultDeps{Repository: encryptedfile.New(cfg.Paths, keys), Files: sourcefile.New()}), nil
}

func testCompose(cfg app.ConfigLoader, vault app.VaultOpener) app.ComposeUseCases {
	source := vaultsource.New(vault.ListEnvs)
	return app.NewComposeUseCases(app.ComposeDeps{
		Envs:       source,
		Catalog:    source,
		Files:      envfile.New(),
		Exports:    envfile.New(),
		Gitignore:  gitignore.New(),
		Selections: selectionfile.New(cfg.Dir),
		Templates:  templatefile.New(),
	})
}

func (ta *testApp) runWith(cmd *cobra.Command, args ...string) int {
	ta.Out.Reset()
	ta.Err.Reset()
	return run(context.Background(), ta.App, newRootCmd(ta.App, cmd), args)
}

func (ta *testApp) initVault(t *testing.T) {
	t.Helper()
	uc, err := ta.App.Vault()
	if err != nil {
		t.Fatalf("Vault: %v", err)
	}
	if _, err := uc.InitVault.Execute(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
}

func (ta *testApp) seed(t *testing.T, envs ...vault.Env) {
	t.Helper()
	cfg := ta.Config(t)
	keys, err := keystore.WithOverride(cfg.KeyOverride, keystore.NewFile(cfg.Paths.Key))
	if err != nil {
		t.Fatalf("keys: %v", err)
	}
	ta.initVault(t)
	repo := encryptedfile.New(cfg.Paths, keys)
	for _, env := range envs {
		err := repo.Update(context.Background(), func(s *vault.Snapshot) error {
			_, err := s.Create(env, vault.Clock(nil).Now())
			return err
		})
		if err != nil {
			t.Fatalf("Create(%s): %v", env.Name, err)
		}
	}
}

func failingCmd(err error) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "falha",
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return err },
	}
	cmd.Flags().Bool(presenter.JSONFlag, false, "saída JSON")
	return cmd
}

var rootCommandNames = []string{
	"init", "list", "show", "get", "set", "unset", "new", "edit", "import",
	"rename", "copy", "delete", "plan", "load", "selection", "exec", "shell", "shell-init", "skill",
	"key", "completion",
}

type fakeTUI struct {
	calls   int
	session app.TUISession
	err     error
}

func (f *fakeTUI) run(_ context.Context, session app.TUISession) error {
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
	if code := ta.Run("--help"); code != presenter.ExitOK {
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
			ta.SetTerminal(tt.stdin, tt.stdout)
			if code := ta.Run(); code != presenter.ExitOK {
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
	ta.SetTerminal(true, true)
	for _, args := range [][]string{{"--version"}, {"--help"}, {"completion", "bash"}} {
		if code := ta.Run(args...); code != presenter.ExitOK {
			t.Fatalf("%v: code = %d", args, code)
		}
	}
	if fake.calls != 0 {
		t.Fatalf("TUI aberta %d vezes", fake.calls)
	}
}

func TestRootWithoutTUIWiringPrintsHelp(t *testing.T) {
	ta := newTestApp(t)
	ta.SetTerminal(true, true)
	if code := ta.Run(); code != presenter.ExitOK || !strings.Contains(ta.Out.String(), "Available Commands") {
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
	ta.SetTerminal(true, true)
	if code := ta.Run(); code != presenter.ExitOK {
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
	ta.SetTerminal(true, true)
	if code := ta.Run(); code != presenter.ExitOK {
		t.Fatalf("code = %d stderr = %q", code, ta.Err.String())
	}
	session := fake.session
	if session.Export != (app.ShellExport{File: "/tmp/envault.x/exports", Dialect: "fish"}) || !session.Export.Active() {
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
		{name: "cancelado", err: context.Canceled, want: presenter.ExitCanceled},
		{name: "cofre", err: vault.ErrNotInitialized, want: presenter.ExitNotInitialized},
		{name: "genérico", err: errors.New("terminal indisponível"), want: presenter.ExitError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := newTestApp(t)
			ta.withTUI(tt.err)
			ta.SetTerminal(true, true)
			if code := ta.Run(); code != tt.want {
				t.Fatalf("code = %d, esperado %d", code, tt.want)
			}
		})
	}
}

func TestRootTUIConfigErrorSkipsTUI(t *testing.T) {
	ta := newTestApp(t)
	fake := ta.withTUI(nil)
	ta.SetTerminal(true, true)
	ta.App.LoadConfig = func() (config.Config, error) { return config.Config{}, errors.New("boom") }
	if code := ta.Run(); code != presenter.ExitError || fake.calls != 0 {
		t.Fatalf("code = %d calls = %d", code, fake.calls)
	}
	t.Setenv(config.EnvKey, "curta")
	ta.App.LoadConfig = config.FromOS
	if code := ta.Run(); code != presenter.ExitValidation || fake.calls != 0 {
		t.Fatalf("chave inválida: code = %d calls = %d", code, fake.calls)
	}
}

func TestVersionFlag(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.Run("--version"); code != presenter.ExitOK {
		t.Fatalf("code = %d", code)
	}
	if ta.Out.String() != "test\n" {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestRootWithoutArgsPrintsHelp(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.Run(); code != presenter.ExitOK {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(ta.Out.String(), "envault") || !strings.Contains(ta.Out.String(), "completion") {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestUsageErrors(t *testing.T) {
	cases := [][]string{
		{"--nao-existe"},
		{"nao-existe"},
		{"completion"},
		{"completion", "tcsh"},
	}
	for _, args := range cases {
		ta := newTestApp(t)
		if code := ta.Run(args...); code != presenter.ExitUsage {
			t.Errorf("%v: code = %d, want %d (stderr %q)", args, code, presenter.ExitUsage, ta.Err.String())
		}
		if !strings.Contains(ta.Err.String(), "envault:") {
			t.Errorf("%v: stderr = %q", args, ta.Err.String())
		}
		if ta.Out.Len() != 0 {
			t.Errorf("%v: stdout not empty: %q", args, ta.Out.String())
		}
	}
}

func TestCompletionShells(t *testing.T) {
	for _, shell := range completionShells {
		ta := newTestApp(t)
		if code := ta.Run("completion", shell); code != presenter.ExitOK {
			t.Fatalf("%s: code = %d (%s)", shell, code, ta.Err.String())
		}
		if !strings.Contains(ta.Out.String(), "envault") {
			t.Fatalf("%s: script does not mention envault", shell)
		}
	}
}

func TestErrorHumanMode(t *testing.T) {
	ta := newTestApp(t)
	code := ta.runWith(failingCmd(vault.ErrEnvNotFound), "falha")
	if code != presenter.ExitEnvNotFound {
		t.Fatalf("code = %d", code)
	}
	if ta.Out.Len() != 0 {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
	if !strings.Contains(ta.Err.String(), vault.ErrEnvNotFound.Error()) {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
}

func TestErrorJSONEnvelope(t *testing.T) {
	ta := newTestApp(t)
	code := ta.runWith(failingCmd(vault.ErrEnvNotFound), "falha", "--json")
	if code != presenter.ExitEnvNotFound {
		t.Fatalf("code = %d", code)
	}
	envelope := clitest.DecodeEnvelope(t, ta.Out.Bytes())
	if envelope.SchemaVersion != 1 || envelope.Error.Code != presenter.ExitEnvNotFound || envelope.Error.Message == "" {
		t.Fatalf("envelope = %+v", envelope)
	}
	if ta.Err.Len() != 0 {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
}

func TestUsageErrorWithJSONFlag(t *testing.T) {
	ta := newTestApp(t)
	code := ta.runWith(failingCmd(nil), "falha", "--json", "--nao-existe")
	if code != presenter.ExitUsage {
		t.Fatalf("code = %d", code)
	}
	if envelope := clitest.DecodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != presenter.ExitUsage {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestAlwaysJSONCommand(t *testing.T) {
	ta := newTestApp(t)
	cmd := presenter.AlwaysJSON(&cobra.Command{
		Use:  "plano",
		RunE: func(*cobra.Command, []string) error { return vault.ErrNotInitialized },
	})
	if code := ta.runWith(cmd, "plano"); code != presenter.ExitNotInitialized {
		t.Fatalf("code = %d", code)
	}
	if envelope := clitest.DecodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != presenter.ExitNotInitialized {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestSilentExitCode(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.runWith(failingCmd(presenter.ChildExited(composeusecase.RunCommandResult{ExitCode: 42})), "falha", "--json"); code != 42 {
		t.Fatalf("code = %d", code)
	}
	if ta.Out.Len() != 0 || ta.Err.Len() != 0 {
		t.Fatalf("unexpected output: %q %q", ta.Out.String(), ta.Err.String())
	}
}

func TestCommandReceivesApp(t *testing.T) {
	ta := newTestApp(t)
	ta.In.WriteString("entrada")
	echoCmd := func(a *app.App) *cobra.Command {
		return &cobra.Command{
			Use: "eco",
			RunE: func(*cobra.Command, []string) error {
				var buf bytes.Buffer
				if _, err := buf.ReadFrom(a.Stdin); err != nil {
					return err
				}
				if err := a.Presenter().Infof("lido %d bytes", buf.Len()); err != nil {
					return err
				}
				return a.Presenter().Script(buf.String())
			},
		}
	}
	if code := ta.runWith(echoCmd(ta.App), "eco"); code != presenter.ExitOK {
		t.Fatalf("code = %d", code)
	}
	if ta.Out.String() != "entrada" || ta.Err.String() != "lido 7 bytes\n" {
		t.Fatalf("out %q err %q", ta.Out.String(), ta.Err.String())
	}
}
