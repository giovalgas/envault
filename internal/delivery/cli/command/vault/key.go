package vault

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
)

func NewKeyCmd(a *app.App) *cobra.Command {
	keyCmd := &cobra.Command{
		Use:   "key",
		Short: "Gerencia onde a chave do cofre fica guardada",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	keyCmd.AddCommand(keyNewMigrateCmd(a))
	return keyCmd
}

func keyNewMigrateCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Move a chave do arquivo key para o keychain do sistema e remove o arquivo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			migrate, err := a.KeyMigration()
			if err != nil {
				return err
			}
			result, err := migrate.Execute(cmd.Context())
			if err != nil {
				return err
			}
			return a.Presenter().KeyMigrated(result)
		},
	}
}
