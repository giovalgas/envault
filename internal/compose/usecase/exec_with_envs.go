package usecase

import (
	"context"
	"runtime"
	"strings"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type ExecWithEnvsInput struct {
	Envs     []string
	Template *domain.Template
	Options  domain.Options
	Environ  []string
}

type ExecWithEnvsResult struct {
	Plan    domain.Plan
	Environ []string
}

type ExecWithEnvs struct {
	envs EnvReader
}

func NewExecWithEnvs(envs EnvReader) *ExecWithEnvs {
	return &ExecWithEnvs{envs: envs}
}

func (uc *ExecWithEnvs) Execute(ctx context.Context, in ExecWithEnvsInput) (ExecWithEnvsResult, error) {
	plan, err := combine(ctx, uc.envs, in.Envs, in.Template, in.Options)
	if err != nil {
		return ExecWithEnvsResult{}, err
	}
	return ExecWithEnvsResult{Plan: plan, Environ: overlayEnviron(in.Environ, plan.Pairs())}, nil
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
