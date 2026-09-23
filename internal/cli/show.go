package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/vault"
)

type showEnvelope struct {
	SchemaVersion int       `json:"schema_version"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Tags          []string  `json:"tags"`
	Keys          []string  `json:"keys"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func newShowCmd(app *App) *cobra.Command {
	var asJSON bool
	showCmd := &cobra.Command{
		Use:   "show <env>",
		Short: "Mostra metadados e nomes das chaves de uma env, sem valores",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := app.OpenVault()
			if err != nil {
				return err
			}
			env, err := v.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(app.Stdout, showBuildEnvelope(env))
			}
			showPrintHuman(app, env)
			return nil
		},
	}
	showCmd.Flags().BoolVar(&asJSON, jsonFlag, false, "saída em JSON")
	return showCmd
}

func showBuildEnvelope(env vault.Env) showEnvelope {
	return showEnvelope{
		SchemaVersion: SchemaVersion,
		Name:          env.Name,
		Description:   env.Description,
		Tags:          listNonNil(env.Tags),
		Keys:          listNonNil(env.Keys()),
		CreatedAt:     env.CreatedAt,
		UpdatedAt:     env.UpdatedAt,
	}
}

func showPrintHuman(app *App, env vault.Env) {
	fmt.Fprintf(app.Stdout, "%s\n", env.Name)
	if env.Description != "" {
		fmt.Fprintf(app.Stdout, "  descrição: %s\n", env.Description)
	}
	if len(env.Tags) > 0 {
		fmt.Fprintf(app.Stdout, "  tags: %s\n", strings.Join(env.Tags, ", "))
	}
	fmt.Fprintf(app.Stdout, "  chaves: %s\n", strings.Join(env.Keys(), ", "))
	fmt.Fprintf(app.Stdout, "  criada em: %s\n", env.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(app.Stdout, "  atualizada em: %s\n", env.UpdatedAt.Format(time.RFC3339))
}
