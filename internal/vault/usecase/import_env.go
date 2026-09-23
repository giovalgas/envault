package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type ImportEnvInput struct {
	Name        string
	From        EnvSource
	Description *string
}

type ImportEnvResult struct {
	Env      domain.Env
	Replaced bool
}

type ImportEnv struct {
	repo  domain.EnvRepository
	clock domain.Clock
}

func NewImportEnv(repo domain.EnvRepository, clock domain.Clock) *ImportEnv {
	return &ImportEnv{repo: repo, clock: clock}
}

func (uc *ImportEnv) Execute(ctx context.Context, in ImportEnvInput) (ImportEnvResult, error) {
	if err := domain.ValidateName(in.Name); err != nil {
		return ImportEnvResult{}, err
	}
	env, err := in.From.load()
	if err != nil {
		return ImportEnvResult{}, err
	}
	env.Name = in.Name
	if in.Description != nil {
		env.Description = *in.Description
	}
	var result ImportEnvResult
	err = uc.repo.Update(ctx, func(s *domain.Snapshot) error {
		result.Replaced = s.Has(in.Name)
		stored, err := s.Put(env, uc.clock.Now())
		result.Env = stored
		return err
	})
	if err != nil {
		return ImportEnvResult{}, err
	}
	return result, nil
}
