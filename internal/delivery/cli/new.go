package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	vault "github.com/giovalgas/envault/internal/vault/domain"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
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
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			in := vaultusecase.CreateEnvInput{
				Env:                 vault.Env{Name: args[0], Description: description, Tags: newCleanTags(tags)},
				OverrideDescription: cmd.Flags().Changed(newFlagDescription),
				OverrideTags:        cmd.Flags().Changed(newFlagTags),
			}
			if fromFile != "" {
				source := importFileSource(fromFile)
				in.From = &source
			}
			result, err := uc.CreateEnv.Execute(cmd.Context(), in)
			if err != nil {
				return editorExitCode(err)
			}
			if !result.Created {
				return withExitCode(ExitCanceled, ErrCanceled)
			}
			return app.Infof("env %q criada com %d chave(s)", result.Env.Name, len(result.Env.Vars))
		},
	}
	newCmd.Flags().StringVar(&description, newFlagDescription, "", "descrição da env")
	newCmd.Flags().StringSliceVar(&tags, newFlagTags, nil, "tags separadas por vírgula")
	newCmd.Flags().StringVar(&fromFile, newFlagFromFile, "", "cria a partir de um arquivo .env, sem abrir o editor")
	return newCmd
}

func editorExitCode(err error) error {
	if errors.Is(err, vault.ErrEditCanceled) {
		return withExitCode(ExitCanceled, err)
	}
	return err
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
