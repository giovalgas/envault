package compose_test

import (
	"context"
	"testing"

	"github.com/giovalgas/envault/internal/compose/infra/envfile"
	"github.com/giovalgas/envault/internal/compose/infra/gitignore"
	"github.com/giovalgas/envault/internal/compose/infra/process"
	"github.com/giovalgas/envault/internal/compose/infra/selectionfile"
	"github.com/giovalgas/envault/internal/compose/infra/templatefile"
	"github.com/giovalgas/envault/internal/compose/infra/vaultsource"
	"github.com/giovalgas/envault/internal/delivery/cli"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/clitest"
	"github.com/giovalgas/envault/internal/shared/config"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	"github.com/giovalgas/envault/internal/vault/infra/sourcefile"
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
		Compose: testCompose,
	}
}

func testComposeVault(cfg config.Config) (testVault, error) {
	keys, err := keystore.WithOverride(cfg.KeyOverride, keystore.NewFile(cfg.Paths.Key))
	if err != nil {
		return testVault{}, err
	}
	repo := encryptedfile.New(cfg.Paths, keys)
	uc := app.NewVaultUseCases(app.VaultDeps{Repository: repo, Files: sourcefile.New()})
	return testVault{repo: repo, uc: uc}, nil
}

func testCompose(cfg app.ConfigLoader, vault app.VaultOpener) app.ComposeUseCases {
	source := vaultsource.New(vault.ListEnvs)
	return app.NewComposeUseCases(app.ComposeDeps{
		Envs:        source,
		Catalog:     source,
		Files:       envfile.New(),
		Exports:     envfile.New(),
		Gitignore:   gitignore.New(),
		Selections:  selectionfile.New(cfg.Dir),
		Templates:   templatefile.New(),
		Environment: process.New(),
		Processes:   process.New(),
	})
}

func (ta *testApp) seed(t *testing.T, envs ...vault.Env) {
	t.Helper()
	v, err := testComposeVault(ta.Config(t))
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
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
