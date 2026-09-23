package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/vault"
)

var errSetMissingEquals = errors.New("informe KEY=VALUE ou uma única KEY para ler do stdin")

type setPair struct {
	Key   string
	Value string
}

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
			v, err := app.OpenVault()
			if err != nil {
				return err
			}
			_, err = v.Modify(cmd.Context(), name, func(env *vault.Env) error {
				for _, p := range pairs {
					if err := vault.ValidateKey(p.Key); err != nil {
						return err
					}
					env.Set(p.Key, p.Value)
				}
				return nil
			})
			if err != nil {
				return err
			}
			app.Infof("definida(s) %d chave(s) em %q", len(pairs), name)
			return nil
		},
	}
}

func setParseArgs(app *App, args []string) ([]setPair, error) {
	if len(args) == 1 && !strings.Contains(args[0], "=") {
		value, err := setReadStdinValue(app)
		if err != nil {
			return nil, err
		}
		return []setPair{{Key: args[0], Value: value}}, nil
	}
	pairs := make([]setPair, 0, len(args))
	for _, arg := range args {
		eq := strings.IndexByte(arg, '=')
		if eq < 0 {
			return nil, usageError(fmt.Errorf("%w: %q", errSetMissingEquals, arg))
		}
		pairs = append(pairs, setPair{Key: arg[:eq], Value: arg[eq+1:]})
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
