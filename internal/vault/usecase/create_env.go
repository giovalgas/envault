package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type CreateEnvInput struct {
	Env                 domain.Env
	From                *EnvSource
	OverrideDescription bool
	OverrideTags        bool
}

type CreateEnvResult struct {
	Env     domain.Env
	Created bool
}

type CreateEnv struct {
	repo   domain.EnvRepository
	editor domain.Editor
	clock  domain.Clock
}

func NewCreateEnv(repo domain.EnvRepository, editor domain.Editor, clock domain.Clock) *CreateEnv {
	return &CreateEnv{repo: repo, editor: editor, clock: clock}
}

func (uc *CreateEnv) Execute(ctx context.Context, in CreateEnvInput) (CreateEnvResult, error) {
	if err := in.Env.Validate(); err != nil {
		return CreateEnvResult{}, err
	}
	if err := ensureAbsent(ctx, uc.repo, in.Env.Name); err != nil {
		return CreateEnvResult{}, err
	}
	env, ok, err := uc.build(ctx, in)
	if err != nil || !ok {
		return CreateEnvResult{}, err
	}
	created, err := storeNewEnv(ctx, uc.repo, uc.clock, env)
	if err != nil {
		return CreateEnvResult{}, err
	}
	return CreateEnvResult{Env: created, Created: true}, nil
}

func (uc *CreateEnv) build(ctx context.Context, in CreateEnvInput) (domain.Env, bool, error) {
	if in.From == nil {
		result, err := uc.editor.Edit(ctx, in.Env, true)
		if err != nil || !result.Changed {
			return domain.Env{}, false, err
		}
		return result.Env, true, nil
	}
	env, err := in.From.load()
	if err != nil {
		return domain.Env{}, false, err
	}
	env.Name = in.Env.Name
	if in.OverrideDescription {
		env.Description = in.Env.Description
	}
	if in.OverrideTags {
		env.Tags = in.Env.Tags
	}
	return env, true, nil
}
