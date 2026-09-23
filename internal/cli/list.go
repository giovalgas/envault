package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/vault"
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
			v, err := app.OpenVault()
			if err != nil {
				return err
			}
			envs, err := v.List(cmd.Context())
			if err != nil {
				return err
			}
			filtered := listFilter(envs, search, tag)
			if asJSON {
				return writeJSON(app.Stdout, listBuildEnvelope(filtered))
			}
			listPrintHuman(app, filtered)
			return nil
		},
	}
	listCmd.Flags().StringVar(&search, "search", "", "filtra por nome, descrição, tag ou nome de chave")
	listCmd.Flags().StringVar(&tag, "tag", "", "filtra por tag exata")
	listCmd.Flags().BoolVar(&asJSON, jsonFlag, false, "saída em JSON")
	return listCmd
}

func listFilter(envs []vault.Env, search, tag string) []vault.Env {
	out := make([]vault.Env, 0, len(envs))
	for _, e := range envs {
		if tag != "" && !e.HasTag(tag) {
			continue
		}
		if !listMatchesSearch(e, search) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func listMatchesSearch(e vault.Env, search string) bool {
	if search == "" {
		return true
	}
	needle := strings.ToLower(search)
	if strings.Contains(strings.ToLower(e.Name), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Description), needle) {
		return true
	}
	for _, t := range e.Tags {
		if strings.Contains(strings.ToLower(t), needle) {
			return true
		}
	}
	for _, k := range e.Keys() {
		if strings.Contains(strings.ToLower(k), needle) {
			return true
		}
	}
	return false
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

func listPrintHuman(app *App, envs []vault.Env) {
	if len(envs) == 0 {
		app.Infof("nenhuma env encontrada")
		return
	}
	for _, e := range envs {
		fmt.Fprintf(app.Stdout, "%s\n", e.Name)
		if e.Description != "" {
			fmt.Fprintf(app.Stdout, "  descrição: %s\n", e.Description)
		}
		if len(e.Tags) > 0 {
			fmt.Fprintf(app.Stdout, "  tags: %s\n", strings.Join(e.Tags, ", "))
		}
		fmt.Fprintf(app.Stdout, "  chaves: %s\n", strings.Join(e.Keys(), ", "))
		fmt.Fprintf(app.Stdout, "  atualizado em: %s\n", e.UpdatedAt.Format(time.RFC3339))
	}
}
