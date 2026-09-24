package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type CreateEnvInput struct {
	Name                string
	Description         string
	Tags                []string
	FromFile            string
	OverrideDescription bool
	OverrideTags        bool
}

type CreateEnvResult struct {
	Env     EnvView
	Created bool
}

type CreateEnv struct {
	repo   domain.EnvRepository
	editor domain.Editor
	files  EnvFileReader
	clock  domain.Clock
}

func NewCreateEnv(repo domain.EnvRepository, editor domain.Editor, files EnvFileReader, clock domain.Clock) *CreateEnv {
	return &CreateEnv{repo: repo, editor: editor, files: files, clock: clock}
}

func (uc *CreateEnv) Execute(ctx context.Context, in CreateEnvInput) (CreateEnvResult, error) {
	initial := domain.Env{Name: in.Name, Description: in.Description, Tags: in.Tags}
	if err := initial.Validate(); err != nil {
		return CreateEnvResult{}, err
	}
	if err := ensureAbsent(ctx, uc.repo, initial.Name); err != nil {
		return CreateEnvResult{}, err
	}
	env, ok, err := uc.build(ctx, in, initial)
	if err != nil || !ok {
		return CreateEnvResult{}, err
	}
	created, err := storeNewEnv(ctx, uc.repo, uc.clock, env)
	if err != nil {
		return CreateEnvResult{}, err
	}
	return CreateEnvResult{Env: envView(created), Created: true}, nil
}

func (uc *CreateEnv) build(ctx context.Context, in CreateEnvInput, initial domain.Env) (domain.Env, bool, error) {
	if in.FromFile == "" {
		result, err := uc.editor.Edit(ctx, initial, true)
		if err != nil || !result.Changed {
			return domain.Env{}, false, err
		}
		return result.Env, true, nil
	}
	env, err := readEnvFile(uc.files, "", in.FromFile)
	if err != nil {
		return domain.Env{}, false, err
	}
	env.Name = initial.Name
	if in.OverrideDescription {
		env.Description = initial.Description
	}
	if in.OverrideTags {
		env.Tags = initial.Tags
	}
	return env, true, nil
}
