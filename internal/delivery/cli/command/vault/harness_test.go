package vault_test

import (
	"context"
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/clitest"
	"github.com/giovalgas/envault/internal/shared/config"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/editor"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	"github.com/giovalgas/envault/internal/vault/infra/sourcefile"
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

func testWiring(h *clitest.Harness) app.Wiring {
	return app.Wiring{
		Vault: func(cfg config.Config, streams app.Streams) (app.VaultUseCases, error) {
			composed, err := testComposeVault(h, cfg, streams)
			return composed.uc, err
		},
		KeyMigration: testKeyMigration,
	}
}

func testKeyStore(h *clitest.Harness, cfg config.Config) vault.KeyStore {
	file := keystore.NewFile(cfg.Paths.Key)
	if !h.Keychain {
		return file
	}
	return keystore.Chain{file, keystore.NewKeyring(keystore.Account(cfg.Paths.Dir))}
}

func testComposeVault(h *clitest.Harness, cfg config.Config, streams app.Streams) (testVault, error) {
	keys, err := keystore.WithOverride(cfg.KeyOverride, testKeyStore(h, cfg))
	if err != nil {
		return testVault{}, err
	}
	repo := encryptedfile.New(cfg.Paths, keys)
	uc := app.NewVaultUseCases(app.VaultDeps{
		Repository: repo,
		Editor: editor.New(editor.Options{
			RuntimeDir: cfg.RuntimeDir,
			Stdin:      streams.Stdin,
			Stdout:     streams.Stdout,
			Stderr:     streams.Stderr,
		}),
		Sessions: editor.NewInteractive(editor.Options{RuntimeDir: cfg.RuntimeDir}),
		Files:    sourcefile.New(),
	})
	return testVault{repo: repo, uc: uc}, nil
}

func testKeyMigration(cfg config.Config) *vaultusecase.MigrateKey {
	return vaultusecase.NewMigrateKey(
		keystore.NewFile(cfg.Paths.Key),
		keystore.NewKeyring(keystore.Account(cfg.Paths.Dir)),
		func(keys vault.KeyStore) vault.EnvRepository { return encryptedfile.New(cfg.Paths, keys) },
	)
}

func (ta *testApp) vault(t *testing.T) testVault {
	t.Helper()
	composed, err := testComposeVault(ta.Harness, ta.Config(t), ta.Streams())
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
	if _, err := v.uc.InitVault.Execute(context.Background()); err != nil {
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
