package vault

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

func NewSetCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "set <env> KEY=VALUE...",
		Short: "Define valores de chaves numa env",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			pairs, err := setParseArgs(a, args[1:])
			if err != nil {
				return err
			}
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			if _, err := uc.SetValues.Execute(cmd.Context(), name, pairs); err != nil {
				return err
			}
			return a.Presenter().ValuesSet(name, len(pairs))
		},
	}
}

func setParseArgs(a *app.App, args []string) ([]vaultusecase.VarView, error) {
	if len(args) == 1 && !strings.Contains(args[0], "=") {
		value, err := setReadStdinValue(a)
		if err != nil {
			return nil, err
		}
		return []vaultusecase.VarView{{Key: args[0], Value: value}}, nil
	}
	pairs := make([]vaultusecase.VarView, 0, len(args))
	for _, arg := range args {
		eq := strings.IndexByte(arg, '=')
		if eq < 0 {
			return nil, presenter.MissingEquals(arg)
		}
		pairs = append(pairs, vaultusecase.VarView{Key: arg[:eq], Value: arg[eq+1:]})
	}
	return pairs, nil
}

func setReadStdinValue(a *app.App) (string, error) {
	data, err := io.ReadAll(a.Stdin)
	if err != nil {
		return "", presenter.ReadStdinError(err)
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}
