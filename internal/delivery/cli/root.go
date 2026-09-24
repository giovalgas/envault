package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	composecmd "github.com/giovalgas/envault/internal/delivery/cli/command/compose"
	skillcmd "github.com/giovalgas/envault/internal/delivery/cli/command/skill"
	vaultcmd "github.com/giovalgas/envault/internal/delivery/cli/command/vault"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
)

var completionShells = []string{"bash", "zsh", "fish", "powershell"}

func Execute(ctx context.Context, a *app.App, args []string) int {
	return run(ctx, a, newRootCmd(a, commands(a)...), args)
}

func commands(a *app.App) []*cobra.Command {
	return []*cobra.Command{
		vaultcmd.NewInitCmd(a),
		vaultcmd.NewListCmd(a),
		vaultcmd.NewShowCmd(a),
		vaultcmd.NewGetCmd(a),
		vaultcmd.NewSetCmd(a),
		vaultcmd.NewUnsetCmd(a),
		vaultcmd.NewNewCmd(a),
		vaultcmd.NewEditCmd(a),
		vaultcmd.NewImportCmd(a),
		vaultcmd.NewRenameCmd(a),
		vaultcmd.NewCopyCmd(a),
		vaultcmd.NewDeleteCmd(a),
		composecmd.NewPlanCmd(a),
		composecmd.NewLoadCmd(a),
		composecmd.NewSelectionCmd(a),
		composecmd.NewExecCmd(a),
		composecmd.NewShellCmd(a),
		composecmd.NewShellInitCmd(a),
		skillcmd.NewSkillCmd(a),
		vaultcmd.NewKeyCmd(a),
		newCompletionCmd(a),
	}
}

func newRootCmd(a *app.App, subcommands ...*cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:               "envault",
		Short:             "Cofre local criptografado de variáveis de ambiente",
		Long:              "envault guarda envs nomeadas num cofre local criptografado e monta o .env de projetos sem expor valores.",
		Version:           a.Version,
		Args:              cobra.NoArgs,
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !rootOpensTUI(a) {
				return cmd.Help()
			}
			session, err := a.TUISession()
			if err != nil {
				return err
			}
			return a.Wire.TUI(cmd.Context(), session)
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(subcommands...)
	return root
}

func rootOpensTUI(a *app.App) bool {
	return a.Wire.TUI != nil && a.Interactive()
}

func newCompletionCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:                   "completion bash|zsh|fish|powershell",
		Short:                 "Gera o script de autocompletar para o shell",
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs:             completionShells,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Presenter().Completion(cmd.Root(), args[0])
		},
	}
}

func run(ctx context.Context, a *app.App, root *cobra.Command, args []string) int {
	root.SetArgs(args)
	root.SetIn(a.Stdin)
	root.SetOut(a.Stdout)
	root.SetErr(a.Stderr)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return presenter.UsageError(err)
	})
	wrapArgValidators(root)
	cmd, err := root.ExecuteContextC(ctx)
	if err != nil {
		return a.Presenter().Error(cmd, args, err)
	}
	return presenter.ExitOK
}

func wrapArgValidators(cmd *cobra.Command) {
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(c *cobra.Command, args []string) error {
			if err := validate(c, args); err != nil {
				return presenter.UsageError(err)
			}
			return nil
		}
	}
	for _, sub := range cmd.Commands() {
		wrapArgValidators(sub)
	}
}
