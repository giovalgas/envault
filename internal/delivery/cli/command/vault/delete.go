package vault

import (
	"bufio"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
)

func NewDeleteCmd(a *app.App) *cobra.Command {
	var yes bool
	deleteCmd := &cobra.Command{
		Use:   "delete <env>",
		Short: "Remove uma env do cofre",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !yes {
				if err := deleteConfirm(a, name); err != nil {
					return err
				}
			}
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			if err := uc.DeleteEnv.Execute(cmd.Context(), name); err != nil {
				return err
			}
			return a.Presenter().EnvDeleted(name)
		},
	}
	deleteCmd.Flags().BoolVar(&yes, "yes", false, "pula a confirmação")
	return deleteCmd
}

func deleteConfirm(a *app.App, name string) error {
	if !a.Interactive() {
		return presenter.DeleteNeedsConfirmation()
	}
	if err := a.Presenter().ConfirmDelete(name); err != nil {
		return err
	}
	line, err := bufio.NewReader(a.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return presenter.ReadConfirmationError(err)
	}
	if strings.TrimSpace(line) != name {
		return presenter.ErrCanceled
	}
	return nil
}
