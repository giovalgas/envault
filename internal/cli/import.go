package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/dotenv"
	"github.com/giovalgas/envault/internal/vault"
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
			if err := vault.ValidateName(name); err != nil {
				return err
			}
			env, err := importReadFile(path)
			if err != nil {
				return err
			}
			env.Name = name
			if cmd.Flags().Changed(importFlagDescription) {
				env.Description = description
			}
			v, err := app.OpenVault()
			if err != nil {
				return err
			}
			existed, err := importExists(cmd, v, name)
			if err != nil {
				return err
			}
			stored, err := v.Put(cmd.Context(), env)
			if err != nil {
				return err
			}
			verb := "criada"
			if existed {
				verb = "substituída"
			}
			app.Infof("env %q %s com %d chave(s) de %s", name, verb, len(stored.Vars), path)
			return nil
		},
	}
	importCmd.Flags().StringVar(&description, importFlagDescription, "", "descrição da env, sobrepõe a do arquivo")
	return importCmd
}

func importReadFile(path string) (vault.Env, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return vault.Env{}, fmt.Errorf("ler %s: %w", path, err)
	}
	env, err := dotenv.Parse(data)
	if err != nil {
		return vault.Env{}, fmt.Errorf("%s: %w", path, err)
	}
	return env, nil
}

func importExists(cmd *cobra.Command, v *vault.Vault, name string) (bool, error) {
	_, err := v.Get(cmd.Context(), name)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, vault.ErrEnvNotFound):
		return false, nil
	default:
		return false, err
	}
}
