package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/vault"
)

const (
	newFlagDescription = "description"
	newFlagTags        = "tags"
	newFlagFromFile    = "from-file"
)

func newNewCmd(app *App) *cobra.Command {
	var (
		description string
		tags        []string
		fromFile    string
	)
	newCmd := &cobra.Command{
		Use:   "new <env>",
		Short: "Cria uma env no $VISUAL ou $EDITOR, ou a partir de um arquivo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			initial := vault.Env{Name: args[0], Description: description, Tags: newCleanTags(tags)}
			if err := initial.Validate(); err != nil {
				return err
			}
			v, err := app.OpenVault()
			if err != nil {
				return err
			}
			exists, err := importExists(cmd, v, initial.Name)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("%w: %q", vault.ErrEnvExists, initial.Name)
			}
			env, err := newBuildEnv(cmd, app, initial, fromFile)
			if err != nil {
				return err
			}
			created, err := v.Create(cmd.Context(), env)
			if err != nil {
				return err
			}
			app.Infof("env %q criada com %d chave(s)", created.Name, len(created.Vars))
			return nil
		},
	}
	newCmd.Flags().StringVar(&description, newFlagDescription, "", "descrição da env")
	newCmd.Flags().StringSliceVar(&tags, newFlagTags, nil, "tags separadas por vírgula")
	newCmd.Flags().StringVar(&fromFile, newFlagFromFile, "", "cria a partir de um arquivo .env, sem abrir o editor")
	return newCmd
}

func newBuildEnv(cmd *cobra.Command, app *App, initial vault.Env, fromFile string) (vault.Env, error) {
	if fromFile == "" {
		result, err := editRunEditor(cmd.Context(), app, initial, true)
		if err != nil {
			return vault.Env{}, err
		}
		if !result.Changed {
			return vault.Env{}, withExitCode(ExitCanceled, ErrCanceled)
		}
		return result.Env, nil
	}
	env, err := importReadFile(fromFile)
	if err != nil {
		return vault.Env{}, err
	}
	env.Name = initial.Name
	if cmd.Flags().Changed(newFlagDescription) {
		env.Description = initial.Description
	}
	if cmd.Flags().Changed(newFlagTags) {
		env.Tags = initial.Tags
	}
	return env, nil
}

func newCleanTags(raw []string) []string {
	tags := make([]string, 0, len(raw))
	for _, tag := range raw {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}
