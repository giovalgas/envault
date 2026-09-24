package usecase

import (
	"context"
	"runtime"
	"strings"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type ExecWithEnvsInput struct {
	Envs         []string
	Template     TemplateView
	OnlyTemplate bool
}

type ExecWithEnvsResult struct {
	Plan    PlanView
	Environ []string
}

type ExecWithEnvs struct {
	envs        EnvReader
	environment Environment
}

func NewExecWithEnvs(envs EnvReader, environment Environment) *ExecWithEnvs {
	return &ExecWithEnvs{envs: envs, environment: environment}
}

func (uc *ExecWithEnvs) Execute(ctx context.Context, in ExecWithEnvsInput) (ExecWithEnvsResult, error) {
	plan, err := combine(ctx, uc.envs, in.Envs, in.Template, in.OnlyTemplate)
	if err != nil {
		return ExecWithEnvsResult{}, err
	}
	return ExecWithEnvsResult{Plan: planView(plan), Environ: overlayEnviron(uc.environment.Environ(), plan.Pairs())}, nil
}

func overlayEnviron(base []string, vars []domain.Var) []string {
	override := make(map[string]struct{}, len(vars))
	for _, v := range vars {
		override[environKey(v.Key)] = struct{}{}
	}
	environ := make([]string, 0, len(base)+len(vars))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := override[environKey(key)]; replaced {
			continue
		}
		environ = append(environ, entry)
	}
	for _, v := range vars {
		environ = append(environ, v.Key+"="+v.Value)
	}
	return environ
}

func environKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}
