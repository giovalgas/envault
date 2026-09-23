package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
)

func newShellInitCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "shell-init bash|zsh|fish",
		Short: "Imprime a função de shell que faz o load exportar no terminal atual",
		Long: "shell-init imprime uma função envault para o rc do shell. Com ela, load sem --out e a montagem da TUI " +
			"exportam as variáveis no shell atual por um arquivo temporário privado, apagado ao final, preservando o " +
			"código de saída. Os demais comandos passam direto. Instale com " + composeusecase.ShellInitLine(shellZsh) +
			" no ~/.zshrc, " + composeusecase.ShellInitLine(shellBash) + " no ~/.bashrc ou " +
			composeusecase.ShellInitLine(shellFish) + " no config.fish.",
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs:             shellDialects,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			script, err := app.Compose().RenderShellWrapper.Execute(cmd.Context(), args[0])
			if err != nil {
				return usageError(err)
			}
			_, err = fmt.Fprint(app.Stdout, script)
			return err
		},
	}
}
