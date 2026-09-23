package cli

import (
	"context"

	"github.com/spf13/cobra"
)

var completionShells = []string{"bash", "zsh", "fish", "powershell"}

func Execute(ctx context.Context, app *App, args []string) int {
	return run(ctx, app, newRootCmd(app, commands(app)...), args)
}

func commands(app *App) []*cobra.Command {
	return []*cobra.Command{
		newInitCmd(app),
		newListCmd(app),
		newShowCmd(app),
		newGetCmd(app),
		newSetCmd(app),
		newUnsetCmd(app),
		newNewCmd(app),
		newEditCmd(app),
		newImportCmd(app),
		newRenameCmd(app),
		newCopyCmd(app),
		newDeleteCmd(app),
		newPlanCmd(app),
		newLoadCmd(app),
		newExecCmd(app),
		newShellCmd(app),
		newSkillCmd(app),
		newKeyCmd(app),
		newCompletionCmd(app),
	}
}

func newRootCmd(app *App, subcommands ...*cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:               "envault",
		Short:             "Cofre local criptografado de variáveis de ambiente",
		Long:              "envault guarda envs nomeadas num cofre local criptografado e monta o .env de projetos sem expor valores.",
		Version:           app.Version,
		Args:              cobra.NoArgs,
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !rootOpensTUI(app) {
				return cmd.Help()
			}
			session, err := app.TUISession()
			if err != nil {
				return err
			}
			return app.Wire.TUI(cmd.Context(), session)
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(subcommands...)
	return root
}

func rootOpensTUI(app *App) bool {
	return app.Wire.TUI != nil && app.Interactive()
}

func newCompletionCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:                   "completion bash|zsh|fish|powershell",
		Short:                 "Gera o script de autocompletar para o shell",
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs:             completionShells,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(app.Stdout, true)
			case "zsh":
				return root.GenZshCompletion(app.Stdout)
			case "fish":
				return root.GenFishCompletion(app.Stdout, true)
			default:
				return root.GenPowerShellCompletionWithDesc(app.Stdout)
			}
		},
	}
}

func run(ctx context.Context, app *App, root *cobra.Command, args []string) int {
	root.SetArgs(args)
	root.SetIn(app.Stdin)
	root.SetOut(app.Stdout)
	root.SetErr(app.Stderr)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError(err)
	})
	wrapArgValidators(root)
	cmd, err := root.ExecuteContextC(ctx)
	if err != nil {
		return reportError(app, cmd, args, err)
	}
	return ExitOK
}

func wrapArgValidators(cmd *cobra.Command) {
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(c *cobra.Command, args []string) error {
			if err := validate(c, args); err != nil {
				return usageError(err)
			}
			return nil
		}
	}
	for _, sub := range cmd.Commands() {
		wrapArgValidators(sub)
	}
}
