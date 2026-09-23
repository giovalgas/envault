package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	vault "github.com/giovalgas/envault/internal/vault/domain"
)

var errSetMissingEquals = errors.New("informe KEY=VALUE ou uma única KEY para ler do stdin")

func newSetCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "set <env> KEY=VALUE...",
		Short: "Define valores de chaves numa env",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			pairs, err := setParseArgs(app, args[1:])
			if err != nil {
				return err
			}
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			if _, err := uc.SetValues.Execute(cmd.Context(), name, pairs); err != nil {
				return err
			}
			app.Infof("definida(s) %d chave(s) em %q", len(pairs), name)
			return nil
		},
	}
}

func setParseArgs(app *App, args []string) ([]vault.Var, error) {
	if len(args) == 1 && !strings.Contains(args[0], "=") {
		value, err := setReadStdinValue(app)
		if err != nil {
			return nil, err
		}
		return []vault.Var{{Key: args[0], Value: value}}, nil
	}
	pairs := make([]vault.Var, 0, len(args))
	for _, arg := range args {
		eq := strings.IndexByte(arg, '=')
		if eq < 0 {
			return nil, usageError(fmt.Errorf("%w: %q", errSetMissingEquals, arg))
		}
		pairs = append(pairs, vault.Var{Key: arg[:eq], Value: arg[eq+1:]})
	}
	return pairs, nil
}

func setReadStdinValue(app *App) (string, error) {
	data, err := io.ReadAll(app.Stdin)
	if err != nil {
		return "", fmt.Errorf("ler valor do stdin: %w", err)
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}
