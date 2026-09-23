package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/giovalgas/envault/internal/compose/infra/envfile"
	"github.com/giovalgas/envault/internal/compose/infra/gitignore"
	"github.com/giovalgas/envault/internal/compose/infra/vaultsource"
	"github.com/giovalgas/envault/internal/delivery/cli"
	"github.com/giovalgas/envault/internal/shared/config"
	skillinfra "github.com/giovalgas/envault/internal/skill/infra"
	skillusecase "github.com/giovalgas/envault/internal/skill/usecase"
	"github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/editor"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	"github.com/giovalgas/envault/internal/vault/usecase"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	wiring := cli.Wiring{Vault: wireVault, KeyMigration: wireKeyMigration, Compose: wireCompose, Skill: wireSkill}
	return cli.Execute(ctx, cli.NewApp(version, wiring), os.Args[1:])
}

func wireVault(cfg config.Config, streams cli.Streams) (cli.VaultUseCases, error) {
	keys, err := keystore.WithOverride(cfg.KeyOverride, keyChain(cfg))
	if err != nil {
		return cli.VaultUseCases{}, err
	}
	return cli.NewVaultUseCases(cli.VaultDeps{
		Repository: encryptedfile.New(cfg.Paths, keys),
		Editor: editor.New(editor.Options{
			RuntimeDir: cfg.RuntimeDir,
			Stdin:      streams.Stdin,
			Stdout:     streams.Stdout,
			Stderr:     streams.Stderr,
		}),
	}), nil
}

func wireKeyMigration(cfg config.Config) *usecase.MigrateKey {
	return usecase.NewMigrateKey(keyFile(cfg), keyring(cfg), func(keys domain.KeyStore) domain.EnvRepository {
		return encryptedfile.New(cfg.Paths, keys)
	})
}

func wireCompose(vault cli.VaultOpener) cli.ComposeUseCases {
	return cli.NewComposeUseCases(cli.ComposeDeps{
		Envs:      vaultsource.New(vault.ListEnvs),
		Files:     envfile.New(),
		Gitignore: gitignore.New(),
	})
}

func wireSkill() *skillusecase.InstallSkill {
	return skillusecase.NewInstallSkill(skillinfra.NewFileWriter(), skillinfra.NewHomeDir())
}

func keyChain(cfg config.Config) keystore.Chain {
	return keystore.Chain{keyFile(cfg), keyring(cfg)}
}

func keyFile(cfg config.Config) *keystore.File {
	return keystore.NewFile(cfg.Paths.Key)
}

func keyring(cfg config.Config) *keystore.Keyring {
	return keystore.NewKeyring(keystore.Account(cfg.Paths.Dir))
}
