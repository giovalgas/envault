package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newDeleteCmd(app *App) *cobra.Command {
	var yes bool
	deleteCmd := &cobra.Command{
		Use:   "delete <env>",
		Short: "Remove uma env do cofre",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !yes {
				if err := deleteConfirm(app, name); err != nil {
					return err
				}
			}
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			if err := uc.DeleteEnv.Execute(cmd.Context(), name); err != nil {
				return err
			}
			return app.Infof("env %q removida", name)
		},
	}
	deleteCmd.Flags().BoolVar(&yes, "yes", false, "pula a confirmação")
	return deleteCmd
}

func deleteConfirm(app *App, name string) error {
	if !app.Interactive() {
		return usageError(fmt.Errorf("confirmação necessária: use --yes ou rode num terminal"))
	}
	if err := app.Infof("digite %q para confirmar a exclusão:", name); err != nil {
		return err
	}
	line, err := bufio.NewReader(app.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("ler confirmação: %w", err)
	}
	if strings.TrimSpace(line) != name {
		return ErrCanceled
	}
	return nil
}
