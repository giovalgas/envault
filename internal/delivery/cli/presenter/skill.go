package presenter

import "fmt"

const skillPermissionsNote = "Adicione as regras de permissão sugeridas em ~/.claude/settings.json ou no .claude/settings.json do projeto.\n" +
	"O deny bloqueia a leitura de .env e .env.local. O .env.example continua legível, e a Skill lê esse arquivo para descobrir as chaves do projeto."

func (p *Presenter) SkillInstalled(path, permissions string) error {
	if err := p.Infof("skill instalada em %s", path); err != nil {
		return err
	}
	if err := p.Infof("%s", skillPermissionsNote); err != nil {
		return err
	}
	_, err := fmt.Fprint(p.stderr, permissions)
	return err
}
