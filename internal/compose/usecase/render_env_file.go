package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/compose/domain"
	"github.com/giovalgas/envault/internal/shared/dotenv"
)

type RenderEnvFileInput struct {
	Envs         []string
	Template     TemplateView
	OnlyTemplate bool
}

type RenderEnvFileResult struct {
	Plan    PlanView
	Content string
}

type RenderEnvFile struct {
	envs EnvReader
}

func NewRenderEnvFile(envs EnvReader) *RenderEnvFile {
	return &RenderEnvFile{envs: envs}
}

func (uc *RenderEnvFile) Execute(ctx context.Context, in RenderEnvFileInput) (RenderEnvFileResult, error) {
	plan, err := combine(ctx, uc.envs, in.Envs, in.Template, in.OnlyTemplate)
	if err != nil {
		return RenderEnvFileResult{}, err
	}
	return RenderEnvFileResult{Plan: planView(plan), Content: renderEnvFile(plan.Pairs())}, nil
}

func renderEnvFile(vars []domain.Var) string {
	out := make([]dotenv.Var, len(vars))
	for i, v := range vars {
		out[i] = dotenv.Var(v)
	}
	return string(dotenv.Format(dotenv.Document{Vars: out}))
}
