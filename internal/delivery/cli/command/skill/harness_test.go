package skill_test

import (
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/clitest"
	skillinfra "github.com/giovalgas/envault/internal/skill/infra"
	skillusecase "github.com/giovalgas/envault/internal/skill/usecase"
)

func newTestApp(t *testing.T) *clitest.Harness {
	t.Helper()
	return clitest.New(t, cli.Execute, func(*clitest.Harness) app.Wiring {
		return app.Wiring{Skill: func() *skillusecase.InstallSkill {
			return skillusecase.NewInstallSkill(skillinfra.NewFileWriter(), skillinfra.NewHomeDir())
		}}
	})
}
