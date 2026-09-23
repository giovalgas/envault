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
	"github.com/giovalgas/envault/internal/delivery/tui"
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
	wiring := cli.Wiring{Vault: wireVault, KeyMigration: wireKeyMigration, Compose: wireCompose, Skill: wireSkill, TUI: runTUI}
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
		Sessions: editor.NewInteractive(editor.Options{RuntimeDir: cfg.RuntimeDir}),
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
		Exports:   envfile.New(),
		Gitignore: gitignore.New(),
	})
}

func runTUI(ctx context.Context, session cli.TUISession) error {
	vault, compose := session.Vault, session.Compose
	store := tui.VaultStore{
		ListEnvs:  vault.ListEnvs,
		ShowEnv:   vault.ShowEnv,
		CopyEnv:   vault.CopyEnv,
		RenameEnv: vault.RenameEnv,
		DeleteEnv: vault.DeleteEnv,
	}
	actions := tui.Actions(tui.Deps{
		BeginCreateEnv: vault.BeginCreateEnv,
		BeginEditEnv:   vault.BeginEditEnv,
		ImportEnv:      vault.ImportEnv,
		PlanLoad:       compose.PlanLoad,
		LoadEnvFile:    compose.LoadEnvFile,
		ShellExports:   compose.LoadShellExports,
		ExportFile:     session.Export.File,
		ExportDialect:  session.Export.Dialect,
	})
	return tui.Run(ctx, store, tui.Options{
		Actions:   actions,
		Debug:     session.Debug,
		Input:     session.Stdin,
		Output:    session.Stdout,
		Report:    session.Stderr,
		InitVault: initVaultCmd(vault),
	})
}

func initVaultCmd(vault cli.VaultUseCases) tui.InitVaultFunc {
	return func(ctx context.Context) (bool, string, error) {
		result, err := vault.InitVault.Execute(ctx)
		return result.Created, result.Location, err
	}
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
