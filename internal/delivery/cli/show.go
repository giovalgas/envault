package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	vault "github.com/giovalgas/envault/internal/vault/domain"
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
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			env, err := uc.ShowEnv.Execute(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(app.Stdout, showBuildEnvelope(env))
			}
			return showPrintHuman(app, env)
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

func showPrintHuman(app *App, env vault.Env) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", env.Name)
	if env.Description != "" {
		fmt.Fprintf(&b, "  descrição: %s\n", env.Description)
	}
	if len(env.Tags) > 0 {
		fmt.Fprintf(&b, "  tags: %s\n", strings.Join(env.Tags, ", "))
	}
	fmt.Fprintf(&b, "  chaves: %s\n", strings.Join(env.Keys(), ", "))
	fmt.Fprintf(&b, "  criada em: %s\n", env.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "  atualizada em: %s\n", env.UpdatedAt.Format(time.RFC3339))
	_, err := io.WriteString(app.Stdout, b.String())
	return err
}
