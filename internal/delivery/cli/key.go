package cli

import (
	"context"

	"github.com/spf13/cobra"
)

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
	migrate, err := app.KeyMigration()
	if err != nil {
		return err
	}
	result, err := migrate.Execute(ctx)
	if err != nil {
		return err
	}
	if !result.Migrated {
		app.Infof("a chave já está no keychain (serviço %s, conta %s); nada a migrar", result.Service, result.Account)
		return nil
	}
	app.Infof("chave migrada para o keychain (serviço %s, conta %s); arquivo %s removido", result.Service, result.Account, result.File)
	return nil
}
