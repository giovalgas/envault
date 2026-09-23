package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const importFlagDescription = "description"

func newImportCmd(app *App) *cobra.Command {
	var description string
	importCmd := &cobra.Command{
		Use:   "import <env> <arquivo>",
		Short: "Cria ou substitui uma env a partir de um arquivo .env",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, path := args[0], args[1]
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			in := vaultusecase.ImportEnvInput{Name: name, From: importFileSource(path)}
			if cmd.Flags().Changed(importFlagDescription) {
				in.Description = &description
			}
			result, err := uc.ImportEnv.Execute(cmd.Context(), in)
			if err != nil {
				return err
			}
			verb := "criada"
			if result.Replaced {
				verb = "substituída"
			}
			app.Infof("env %q %s com %d chave(s) de %s", name, verb, len(result.Env.Vars), path)
			return nil
		},
	}
	importCmd.Flags().StringVar(&description, importFlagDescription, "", "descrição da env, sobrepõe a do arquivo")
	return importCmd
}

func importFileSource(path string) vaultusecase.EnvSource {
	return vaultusecase.EnvSource{
		Name: path,
		Read: func() ([]byte, error) {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("ler %s: %w", path, err)
			}
			return data, nil
		},
	}
}
