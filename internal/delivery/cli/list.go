package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	vault "github.com/giovalgas/envault/internal/vault/domain"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

type listEnvelope struct {
	SchemaVersion int            `json:"schema_version"`
	Envs          []listEnvEntry `json:"envs"`
}

type listEnvEntry struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Keys        []string  `json:"keys"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newListCmd(app *App) *cobra.Command {
	var search, tag string
	var asJSON bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Lista as envs do cofre, sem valores",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			filtered, err := uc.ListEnvs.Execute(cmd.Context(), vaultusecase.ListEnvsQuery{Search: search, Tag: tag})
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(app.Stdout, listBuildEnvelope(filtered))
			}
			return listPrintHuman(app, filtered)
		},
	}
	listCmd.Flags().StringVar(&search, "search", "", "filtra por nome, descrição, tag ou nome de chave")
	listCmd.Flags().StringVar(&tag, "tag", "", "filtra por tag exata")
	listCmd.Flags().BoolVar(&asJSON, jsonFlag, false, "saída em JSON")
	return listCmd
}

func listBuildEnvelope(envs []vault.Env) listEnvelope {
	entries := make([]listEnvEntry, len(envs))
	for i, e := range envs {
		entries[i] = listEnvEntry{
			Name:        e.Name,
			Description: e.Description,
			Tags:        listNonNil(e.Tags),
			Keys:        listNonNil(e.Keys()),
			UpdatedAt:   e.UpdatedAt,
		}
	}
	return listEnvelope{SchemaVersion: SchemaVersion, Envs: entries}
}

func listNonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func listPrintHuman(app *App, envs []vault.Env) error {
	if len(envs) == 0 {
		return app.Infof("nenhuma env encontrada")
	}
	var b strings.Builder
	for _, e := range envs {
		fmt.Fprintf(&b, "%s\n", e.Name)
		if e.Description != "" {
			fmt.Fprintf(&b, "  descrição: %s\n", e.Description)
		}
		if len(e.Tags) > 0 {
			fmt.Fprintf(&b, "  tags: %s\n", strings.Join(e.Tags, ", "))
		}
		fmt.Fprintf(&b, "  chaves: %s\n", strings.Join(e.Keys(), ", "))
		fmt.Fprintf(&b, "  atualizado em: %s\n", e.UpdatedAt.Format(time.RFC3339))
	}
	_, err := io.WriteString(app.Stdout, b.String())
	return err
}
