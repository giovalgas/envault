package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/config"
	"github.com/giovalgas/envault/internal/store"
	"github.com/giovalgas/envault/internal/vault"
)

func KeychainKeyStore(cfg config.Config) store.KeyStore {
	return store.Chain{store.NewFile(cfg.Paths.Key), keyKeyring(cfg)}
}

func keyKeyring(cfg config.Config) *store.Keyring {
	return store.NewKeyring(keyAccount(cfg))
}

func keyAccount(cfg config.Config) string {
	abs, err := filepath.Abs(cfg.Paths.Dir)
	if err != nil {
		return filepath.Clean(cfg.Paths.Dir)
	}
	return abs
}

func newKeyCmd(app *App) *cobra.Command {
	keyCmd := &cobra.Command{
		Use:   "key",
		Short: "Gerencia onde a chave do cofre fica guardada",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	keyCmd.AddCommand(keyNewMigrateCmd(app))
	return keyCmd
}

func keyNewMigrateCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Move a chave do arquivo key para o keychain do sistema e remove o arquivo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return keyMigrate(cmd.Context(), app)
		},
	}
}

func keyMigrate(ctx context.Context, app *App) error {
	cfg, err := app.Config()
	if err != nil {
		return err
	}
	file := store.NewFile(cfg.Paths.Key)
	keychain := keyKeyring(cfg)
	key, err := file.Load(ctx)
	if errors.Is(err, store.ErrNoKey) {
		return keyMigrateWithoutFile(ctx, app, cfg, keychain)
	}
	if err != nil {
		return err
	}
	fileKey, err := store.NewStatic(key)
	if err != nil {
		return err
	}
	vaultExists, err := vault.New(cfg.Paths, fileKey).Exists()
	if err != nil {
		return err
	}
	if err := keyVerifyVault(ctx, cfg, fileKey, vaultExists); err != nil {
		return fmt.Errorf("a chave do arquivo %s não abre o cofre, nada foi migrado: %w", file.Path(), err)
	}
	if err := keychain.Save(ctx, key); err != nil {
		return keyKeptError(file, "gravar no keychain", err)
	}
	readBack, err := keychain.Load(ctx)
	if err != nil {
		return keyKeptError(file, "ler de volta do keychain", err)
	}
	if !bytes.Equal(readBack, key) {
		return keyKeptError(file, "ler de volta do keychain", errors.New("a chave lida difere da gravada"))
	}
	if err := keyVerifyVault(ctx, cfg, keychain, vaultExists); err != nil {
		return keyKeptError(file, "abrir o cofre com a chave do keychain", err)
	}
	if err := file.Remove(ctx); err != nil {
		return fmt.Errorf("chave copiada para o keychain, mas o arquivo %s não pôde ser removido: %w", file.Path(), err)
	}
	app.Infof("chave migrada para o keychain (serviço %s, conta %s); arquivo %s removido", keychain.Service(), keychain.Account(), file.Path())
	return nil
}

func keyMigrateWithoutFile(ctx context.Context, app *App, cfg config.Config, keychain *store.Keyring) error {
	_, err := keychain.Load(ctx)
	switch {
	case err == nil:
		app.Infof("a chave já está no keychain (serviço %s, conta %s); nada a migrar", keychain.Service(), keychain.Account())
		return nil
	case errors.Is(err, store.ErrKeyringUnavailable):
		return err
	case errors.Is(err, store.ErrNoKey):
		return fmt.Errorf("%w: arquivo de chave %s não existe; rode envault init", vault.ErrNotInitialized, cfg.Paths.Key)
	default:
		return err
	}
}

func keyVerifyVault(ctx context.Context, cfg config.Config, keys store.KeyStore, vaultExists bool) error {
	if !vaultExists {
		return nil
	}
	_, err := vault.New(cfg.Paths, keys).Load(ctx)
	return err
}

func keyKeptError(file *store.File, step string, err error) error {
	return fmt.Errorf("%s falhou, o arquivo %s foi mantido: %w", step, file.Path(), err)
}
