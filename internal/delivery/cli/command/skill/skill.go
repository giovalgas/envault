package skill

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	skillusecase "github.com/giovalgas/envault/internal/skill/usecase"
)

const skillFlagDir = "dir"

func NewSkillCmd(a *app.App) *cobra.Command {
	skillCmd := &cobra.Command{
		Use:   "skill",
		Short: "Gerencia a Skill do Claude Code embutida no envault",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	skillCmd.AddCommand(skillNewInstallCmd(a))
	return skillCmd
}

func skillNewInstallCmd(a *app.App) *cobra.Command {
	var dir string
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Instala a Skill em ~/.claude/skills/envault/SKILL.md ou no diretório indicado",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := a.Wire.Skill().Execute(cmd.Context(), dir)
			if err != nil {
				return err
			}
			permissions, err := skillusecase.PermissionsJSON()
			if err != nil {
				return err
			}
			return a.Presenter().SkillInstalled(path, permissions)
		},
	}
	installCmd.Flags().StringVar(&dir, skillFlagDir, "", "diretório de skills (padrão ~/.claude/skills)")
	return installCmd
}
