package vault

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	newFlagDescription = "description"
	newFlagTags        = "tags"
	newFlagFromFile    = "from-file"
)

func NewNewCmd(a *app.App) *cobra.Command {
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
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			in := vaultusecase.CreateEnvInput{
				Name:                args[0],
				Description:         description,
				Tags:                newCleanTags(tags),
				FromFile:            fromFile,
				OverrideDescription: cmd.Flags().Changed(newFlagDescription),
				OverrideTags:        cmd.Flags().Changed(newFlagTags),
			}
			result, err := uc.CreateEnv.Execute(cmd.Context(), in)
			if err != nil {
				return presenter.EditorError(err)
			}
			return a.Presenter().EnvCreated(result)
		},
	}
	newCmd.Flags().StringVar(&description, newFlagDescription, "", "descrição da env")
	newCmd.Flags().StringSliceVar(&tags, newFlagTags, nil, "tags separadas por vírgula")
	newCmd.Flags().StringVar(&fromFile, newFlagFromFile, "", "cria a partir de um arquivo .env, sem abrir o editor")
	return newCmd
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
