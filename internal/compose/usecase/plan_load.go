package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type PlanLoadInput struct {
	Envs         []string
	Template     TemplateView
	OnlyTemplate bool
	Target       string
	Dir          string
}

type PlanLoadResult struct {
	Plan   PlanView
	Target TargetView
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
	plan, err := combine(ctx, uc.envs, in.Envs, in.Template, in.OnlyTemplate)
	if err != nil {
		return PlanLoadResult{}, err
	}
	if in.Target == "" {
		return PlanLoadResult{Plan: planView(plan)}, nil
	}
	path := resolveIn(in.Dir, in.Target)
	exists, err := uc.files.Exists(path)
	if err != nil {
		return PlanLoadResult{}, err
	}
	target := domain.Target{Path: path, Exists: exists, Gitignored: uc.gitignore.IsIgnored(path)}
	return PlanLoadResult{Plan: planView(plan), Target: targetView(target)}, nil
}

func combine(ctx context.Context, reader EnvReader, names []string, tmpl TemplateView, only bool) (domain.Plan, error) {
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
	return domain.Resolve(envs, tmpl.toDomain(), domain.Options{OnlyTemplate: only}), nil
}
