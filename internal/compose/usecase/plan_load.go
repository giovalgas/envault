package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type PlanLoadInput struct {
	Envs     []string
	Template *domain.Template
	Options  domain.Options
	Target   string
}

type PlanLoadResult struct {
	Plan   domain.Plan
	Target domain.Target
}

type PlanLoad struct {
	envs      EnvReader
	files     TargetFiles
	gitignore GitignoreChecker
}

func NewPlanLoad(envs EnvReader, files TargetFiles, gitignore GitignoreChecker) *PlanLoad {
	return &PlanLoad{envs: envs, files: files, gitignore: gitignore}
}

func (uc *PlanLoad) Execute(ctx context.Context, in PlanLoadInput) (PlanLoadResult, error) {
	plan, err := combine(ctx, uc.envs, in.Envs, in.Template, in.Options)
	if err != nil {
		return PlanLoadResult{}, err
	}
	exists, err := uc.files.Exists(in.Target)
	if err != nil {
		return PlanLoadResult{}, err
	}
	target := domain.Target{Path: in.Target, Exists: exists, Gitignored: uc.gitignore.IsIgnored(in.Target)}
	return PlanLoadResult{Plan: plan, Target: target}, nil
}

func combine(ctx context.Context, reader EnvReader, names []string, tmpl *domain.Template, opts domain.Options) (domain.Plan, error) {
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, dup := seen[name]; dup {
			return domain.Plan{}, &DuplicateEnvError{Name: name}
		}
		seen[name] = struct{}{}
	}
	envs, err := reader.ReadEnvs(ctx, names)
	if err != nil {
		return domain.Plan{}, err
	}
	return domain.Resolve(envs, tmpl, opts), nil
}
