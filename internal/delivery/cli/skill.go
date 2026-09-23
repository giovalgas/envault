package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/skill/domain"
)

const skillFlagDir = "dir"

const skillPermissionsNote = "Adicione em ~/.claude/settings.json (ou .claude/settings.json do projeto) as regras de permissão sugeridas.\n" +
	"O deny cobre .env e .env.local, mas deixa .env.example legível para a Skill descobrir as chaves do projeto."

func newSkillCmd(app *App) *cobra.Command {
	skillCmd := &cobra.Command{
		Use:   "skill",
		Short: "Gerencia a Skill do Claude Code embutida no envault",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	skillCmd.AddCommand(skillNewInstallCmd(app))
	return skillCmd
}

func skillNewInstallCmd(app *App) *cobra.Command {
	var dir string
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Instala a Skill em ~/.claude/skills/envault/SKILL.md ou no diretório indicado",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return skillInstall(cmd, app, dir)
		},
	}
	installCmd.Flags().StringVar(&dir, skillFlagDir, "", "diretório de skills (padrão ~/.claude/skills)")
	return installCmd
}

func skillInstall(cmd *cobra.Command, app *App, dir string) error {
	path, err := app.Wire.Skill().Execute(cmd.Context(), dir)
	if err != nil {
		return err
	}
	permissions, err := domain.PermissionsJSON()
	if err != nil {
		return err
	}
	app.Infof("skill instalada em %s", path)
	app.Infof("%s", skillPermissionsNote)
	fmt.Fprint(app.Stderr, permissions)
	return nil
}
