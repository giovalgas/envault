package app_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/giovalgas/envault/internal/compose/infra/vaultsource"
	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/cli"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/clitest"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	"github.com/giovalgas/envault/internal/shared/config"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

type testApp struct{ *clitest.Harness }

type testVault struct {
	repo *encryptedfile.Repository
	uc   app.VaultUseCases
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	return &testApp{Harness: clitest.New(t, cli.Execute, testWiring)}
}

func testWiring(*clitest.Harness) app.Wiring {
	return app.Wiring{
		Vault: func(cfg config.Config, _ app.Streams) (app.VaultUseCases, error) {
			composed, err := testComposeVault(cfg)
			return composed.uc, err
		},
		KeyMigration: testKeyMigration,
		Compose:      testCompose,
	}
}

func testComposeVault(cfg config.Config) (testVault, error) {
	keys, err := keystore.WithOverride(cfg.KeyOverride, keystore.NewFile(cfg.Paths.Key))
	if err != nil {
		return testVault{}, err
	}
	repo := encryptedfile.New(cfg.Paths, keys)
	return testVault{repo: repo, uc: app.NewVaultUseCases(app.VaultDeps{Repository: repo})}, nil
}

func testKeyMigration(cfg config.Config) *vaultusecase.MigrateKey {
	return vaultusecase.NewMigrateKey(
		keystore.NewFile(cfg.Paths.Key),
		keystore.NewKeyring(keystore.Account(cfg.Paths.Dir)),
		func(keys vault.KeyStore) vault.EnvRepository { return encryptedfile.New(cfg.Paths, keys) },
	)
}

func testCompose(_ app.ConfigLoader, vault app.VaultOpener) app.ComposeUseCases {
	source := vaultsource.New(vault.ListEnvs)
	return app.NewComposeUseCases(app.ComposeDeps{Envs: source, Catalog: source})
}

func (ta *testApp) vault(t *testing.T) testVault {
	t.Helper()
	composed, err := testComposeVault(ta.Config(t))
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	return composed
}

func (v testVault) Get(ctx context.Context, name string) (vault.Env, error) {
	snapshot, err := v.repo.Load(ctx)
	if err != nil {
		return vault.Env{}, err
	}
	return snapshot.Get(name)
}

func (ta *testApp) seed(t *testing.T, envs ...vault.Env) {
	t.Helper()
	v := ta.vault(t)
	if _, err := v.uc.InitVault.Execute(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for _, env := range envs {
		err := v.repo.Update(context.Background(), func(s *vault.Snapshot) error {
			_, err := s.Create(env, vault.Clock(nil).Now())
			return err
		})
		if err != nil {
			t.Fatalf("Create(%s): %v", env.Name, err)
		}
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
	if presenter.ExitCode(err) != presenter.ExitValidation {
		t.Fatalf("ExitCode = %d", presenter.ExitCode(err))
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
	ta.SetTerminal(true, false)
	if !ta.App.StdinIsTerminal() || ta.App.StdoutIsTerminal() || ta.App.Interactive() {
		t.Fatal("setTerminal(true, false) not honored")
	}
	ta.SetTerminal(true, true)
	if !ta.App.Interactive() {
		t.Fatal("setTerminal(true, true) not interactive")
	}
	if ta.IsTerminal(os.Stderr) {
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
	isTerminal := app.NewApp("test", app.Wiring{}).IsTerminal
	if isTerminal(regular) || isTerminal(&bytes.Buffer{}) {
		t.Fatal("isTerminal true for non-terminal")
	}
}

func TestNewApp(t *testing.T) {
	ta := newTestApp(t)
	a := app.NewApp("1.2.3", ta.App.Wire)
	if a.Stdin != os.Stdin || a.Stdout != os.Stdout || a.Stderr != os.Stderr {
		t.Fatal("NewApp does not use process streams")
	}
	if a.Version != "1.2.3" || a.LoadConfig == nil || a.Wire.Vault == nil || a.Wire.KeyMigration == nil || a.Wire.Compose == nil || a.IsTerminal == nil {
		t.Fatalf("NewApp = %+v", a)
	}
}
