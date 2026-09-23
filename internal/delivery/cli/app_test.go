package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/compose/infra/envfile"
	"github.com/giovalgas/envault/internal/compose/infra/gitignore"
	"github.com/giovalgas/envault/internal/compose/infra/vaultsource"
	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/shared/config"
	skillinfra "github.com/giovalgas/envault/internal/skill/infra"
	skillusecase "github.com/giovalgas/envault/internal/skill/usecase"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/editor"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

type testApp struct {
	App  *App
	Home string
	In   *bytes.Buffer
	Out  *bytes.Buffer
	Err  *bytes.Buffer

	stdinTTY  bool
	stdoutTTY bool
	keychain  bool
}

type testVault struct {
	repo *encryptedfile.Repository
	uc   VaultUseCases
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	home := filepath.Join(t.TempDir(), "envault")
	t.Setenv(config.EnvHome, home)
	t.Setenv(config.EnvKey, "")
	t.Setenv(config.EnvDebug, "")
	ta := &testApp{
		Home: home,
		In:   &bytes.Buffer{},
		Out:  &bytes.Buffer{},
		Err:  &bytes.Buffer{},
	}
	ta.App = &App{
		Stdin:      ta.In,
		Stdout:     ta.Out,
		Stderr:     ta.Err,
		Version:    "test",
		LoadConfig: config.FromOS,
		Wire:       Wiring{Vault: ta.wireVault, KeyMigration: testKeyMigration, Compose: testCompose, Skill: testSkill},
		IsTerminal: ta.isTerminal,
	}
	return ta
}

func (ta *testApp) useKeychain() {
	ta.keychain = true
}

func (ta *testApp) keyStore(cfg config.Config) vault.KeyStore {
	file := keystore.NewFile(cfg.Paths.Key)
	if !ta.keychain {
		return file
	}
	return keystore.Chain{file, keystore.NewKeyring(keystore.Account(cfg.Paths.Dir))}
}

func (ta *testApp) compose(cfg config.Config, streams Streams) (testVault, error) {
	keys, err := keystore.WithOverride(cfg.KeyOverride, ta.keyStore(cfg))
	if err != nil {
		return testVault{}, err
	}
	repo := encryptedfile.New(cfg.Paths, keys)
	uc := NewVaultUseCases(VaultDeps{
		Repository: repo,
		Editor: editor.New(editor.Options{
			RuntimeDir: cfg.RuntimeDir,
			Stdin:      streams.Stdin,
			Stdout:     streams.Stdout,
			Stderr:     streams.Stderr,
		}),
		Sessions: editor.NewInteractive(editor.Options{RuntimeDir: cfg.RuntimeDir}),
	})
	return testVault{repo: repo, uc: uc}, nil
}

func (ta *testApp) wireVault(cfg config.Config, streams Streams) (VaultUseCases, error) {
	composed, err := ta.compose(cfg, streams)
	return composed.uc, err
}

func testKeyMigration(cfg config.Config) *vaultusecase.MigrateKey {
	return vaultusecase.NewMigrateKey(
		keystore.NewFile(cfg.Paths.Key),
		keystore.NewKeyring(keystore.Account(cfg.Paths.Dir)),
		func(keys vault.KeyStore) vault.EnvRepository { return encryptedfile.New(cfg.Paths, keys) },
	)
}

func testSkill() *skillusecase.InstallSkill {
	return skillusecase.NewInstallSkill(skillinfra.NewFileWriter(), skillinfra.NewHomeDir())
}

func testCompose(vault VaultOpener) ComposeUseCases {
	return NewComposeUseCases(ComposeDeps{
		Envs:      vaultsource.New(vault.ListEnvs),
		Files:     envfile.New(),
		Gitignore: gitignore.New(),
	})
}

func (ta *testApp) isTerminal(stream any) bool {
	switch stream {
	case any(ta.In):
		return ta.stdinTTY
	case any(ta.Out):
		return ta.stdoutTTY
	default:
		return false
	}
}

func (ta *testApp) setTerminal(stdin, stdout bool) {
	ta.stdinTTY = stdin
	ta.stdoutTTY = stdout
}

func (ta *testApp) run(args ...string) int {
	ta.Out.Reset()
	ta.Err.Reset()
	return Execute(context.Background(), ta.App, args)
}

func (ta *testApp) runWith(cmd *cobra.Command, args ...string) int {
	ta.Out.Reset()
	ta.Err.Reset()
	return run(context.Background(), ta.App, newRootCmd(ta.App, cmd), args)
}

func (ta *testApp) vault(t *testing.T) testVault {
	t.Helper()
	cfg, err := ta.App.Config()
	if err != nil {
		t.Fatalf("Config: %v", err)
	}
	composed, err := ta.compose(cfg, Streams{Stdin: ta.In, Stdout: ta.Out, Stderr: ta.Err})
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	return composed
}

func (v testVault) Init(ctx context.Context) (bool, error) {
	result, err := v.uc.InitVault.Execute(ctx)
	return result.Created, err
}

func (v testVault) Get(ctx context.Context, name string) (vault.Env, error) {
	return v.uc.ShowEnv.Execute(ctx, name)
}

func (v testVault) Load(ctx context.Context) (*vault.Snapshot, error) {
	return v.repo.Load(ctx)
}

func (v testVault) Create(ctx context.Context, env vault.Env) (vault.Env, error) {
	var created vault.Env
	err := v.repo.Update(ctx, func(s *vault.Snapshot) error {
		var err error
		created, err = s.Create(env, vault.Clock(nil).Now())
		return err
	})
	return created, err
}

func (ta *testApp) initVault(t *testing.T) testVault {
	t.Helper()
	v := ta.vault(t)
	if _, err := v.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return v
}

func (ta *testApp) seed(t *testing.T, envs ...vault.Env) {
	t.Helper()
	v := ta.initVault(t)
	for _, env := range envs {
		if _, err := v.Create(context.Background(), env); err != nil {
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
	cmd.Flags().Bool(jsonFlag, false, "saída JSON")
	return cmd
}

func decodeEnvelope(t *testing.T, data []byte) errorEnvelope {
	t.Helper()
	var envelope errorEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, data)
	}
	return envelope
}

func TestVersionFlag(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.run("--version"); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	if ta.Out.String() != "test\n" {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestRootWithoutArgsPrintsHelp(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.run(); code != ExitOK {
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
		if code := ta.run(args...); code != ExitUsage {
			t.Errorf("%v: code = %d, want %d (stderr %q)", args, code, ExitUsage, ta.Err.String())
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
		if code := ta.run("completion", shell); code != ExitOK {
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
	if code != ExitEnvNotFound {
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
	if code != ExitEnvNotFound {
		t.Fatalf("code = %d", code)
	}
	envelope := decodeEnvelope(t, ta.Out.Bytes())
	if envelope.SchemaVersion != 1 || envelope.Error.Code != ExitEnvNotFound || envelope.Error.Message == "" {
		t.Fatalf("envelope = %+v", envelope)
	}
	if ta.Err.Len() != 0 {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
}

func TestUsageErrorWithJSONFlag(t *testing.T) {
	ta := newTestApp(t)
	code := ta.runWith(failingCmd(nil), "falha", "--json", "--nao-existe")
	if code != ExitUsage {
		t.Fatalf("code = %d", code)
	}
	if envelope := decodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != ExitUsage {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestAlwaysJSONCommand(t *testing.T) {
	ta := newTestApp(t)
	cmd := alwaysJSON(&cobra.Command{
		Use:  "plano",
		RunE: func(*cobra.Command, []string) error { return vault.ErrNotInitialized },
	})
	if code := ta.runWith(cmd, "plano"); code != ExitNotInitialized {
		t.Fatalf("code = %d", code)
	}
	if envelope := decodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != ExitNotInitialized {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestSilentExitCode(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.runWith(failingCmd(withExitCode(42, nil)), "falha", "--json"); code != 42 {
		t.Fatalf("code = %d", code)
	}
	if ta.Out.Len() != 0 || ta.Err.Len() != 0 {
		t.Fatalf("unexpected output: %q %q", ta.Out.String(), ta.Err.String())
	}
}

func TestCommandReceivesApp(t *testing.T) {
	ta := newTestApp(t)
	ta.In.WriteString("entrada")
	newEchoCmd := func(app *App) *cobra.Command {
		return &cobra.Command{
			Use: "eco",
			RunE: func(*cobra.Command, []string) error {
				var buf bytes.Buffer
				if _, err := buf.ReadFrom(app.Stdin); err != nil {
					return err
				}
				if err := app.Infof("lido %d bytes", buf.Len()); err != nil {
					return err
				}
				_, err := app.Stdout.Write(buf.Bytes())
				return err
			},
		}
	}
	if code := ta.runWith(newEchoCmd(ta.App), "eco"); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	if ta.Out.String() != "entrada" || ta.Err.String() != "lido 7 bytes\n" {
		t.Fatalf("out %q err %q", ta.Out.String(), ta.Err.String())
	}
}

func TestVaultUsesEnvaultHome(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a", Vars: []vault.Var{{Key: "SECRET", Value: "supersegredo"}}})
	for _, name := range []string{config.VaultFileName, config.KeyFileName} {
		if _, err := os.Stat(filepath.Join(ta.Home, name)); err != nil {
			t.Fatalf("Stat(%s): %v", name, err)
		}
	}
	env, err := ta.vault(t).Get(context.Background(), "a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value, _ := env.Lookup("SECRET"); value != "supersegredo" {
		t.Fatalf("SECRET = %q", value)
	}
}

func TestVaultRejectsMalformedEnvaultKey(t *testing.T) {
	ta := newTestApp(t)
	t.Setenv(config.EnvKey, "curta")
	_, err := ta.App.Vault()
	if !errors.Is(err, vault.ErrMalformedKey) {
		t.Fatalf("err = %v, want ErrMalformedKey", err)
	}
	if ExitCode(err) != ExitValidation {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
}

func TestVaultConfigError(t *testing.T) {
	ta := newTestApp(t)
	boom := errors.New("boom")
	ta.App.LoadConfig = func() (config.Config, error) { return config.Config{}, boom }
	if _, err := ta.App.Vault(); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if _, err := ta.App.KeyMigration(); !errors.Is(err, boom) {
		t.Fatalf("KeyMigration err = %v, want boom", err)
	}
}

func TestComposeIsLazy(t *testing.T) {
	ta := newTestApp(t)
	boom := errors.New("boom")
	ta.App.LoadConfig = func() (config.Config, error) { return config.Config{}, boom }
	uc := ta.App.Compose()
	if uc.PlanLoad == nil || uc.LoadEnvFile == nil || uc.ExecWithEnvs == nil || uc.RenderShell == nil {
		t.Fatalf("Compose = %+v", uc)
	}
	if _, err := uc.RenderShell.Execute(context.Background(), composeusecase.RenderShellInput{Envs: []string{"a"}}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestKeyMigrationIgnoresEnvaultKey(t *testing.T) {
	ta := newTestApp(t)
	t.Setenv(config.EnvKey, "curta")
	if migrate, err := ta.App.KeyMigration(); err != nil || migrate == nil {
		t.Fatalf("KeyMigration = %v, %v", migrate, err)
	}
}

func TestTerminalDetection(t *testing.T) {
	ta := newTestApp(t)
	if ta.App.StdinIsTerminal() || ta.App.StdoutIsTerminal() || ta.App.Interactive() {
		t.Fatal("buffers reported as terminal")
	}
	ta.setTerminal(true, false)
	if !ta.App.StdinIsTerminal() || ta.App.StdoutIsTerminal() || ta.App.Interactive() {
		t.Fatal("setTerminal(true, false) not honored")
	}
	ta.setTerminal(true, true)
	if !ta.App.Interactive() {
		t.Fatal("setTerminal(true, true) not interactive")
	}
	if ta.isTerminal(os.Stderr) {
		t.Fatal("unknown stream reported as terminal")
	}

	regular, err := os.CreateTemp(t.TempDir(), "notatty")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	t.Cleanup(func() {
		if err := regular.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	if isTerminal(regular) || isTerminal(&bytes.Buffer{}) {
		t.Fatal("isTerminal true for non-terminal")
	}
}

func TestNewApp(t *testing.T) {
	ta := newTestApp(t)
	app := NewApp("1.2.3", ta.App.Wire)
	if app.Stdin != os.Stdin || app.Stdout != os.Stdout || app.Stderr != os.Stderr {
		t.Fatal("NewApp does not use process streams")
	}
	if app.Version != "1.2.3" || app.LoadConfig == nil || app.Wire.Vault == nil || app.Wire.KeyMigration == nil || app.Wire.Compose == nil || app.IsTerminal == nil {
		t.Fatalf("NewApp = %+v", app)
	}
}
