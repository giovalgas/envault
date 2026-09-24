package vault

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const importFlagDescription = "description"

func NewImportCmd(a *app.App) *cobra.Command {
	var description string
	importCmd := &cobra.Command{
		Use:   "import <env> <arquivo>",
		Short: "Cria ou substitui uma env a partir de um arquivo .env",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, path := args[0], args[1]
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			in := vaultusecase.ImportEnvInput{Name: name, Path: path}
			if cmd.Flags().Changed(importFlagDescription) {
				in.Description = &description
			}
			result, err := uc.ImportEnv.Execute(cmd.Context(), in)
			if err != nil {
				return err
			}
			return a.Presenter().EnvImported(name, path, result)
		},
	}
	importCmd.Flags().StringVar(&description, importFlagDescription, "", "descrição da env, sobrepõe a do arquivo")
	return importCmd
}
