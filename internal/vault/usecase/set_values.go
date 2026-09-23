package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type SetValues struct {
	repo  domain.EnvRepository
	clock domain.Clock
}

func NewSetValues(repo domain.EnvRepository, clock domain.Clock) *SetValues {
	return &SetValues{repo: repo, clock: clock}
}

func (uc *SetValues) Execute(ctx context.Context, name string, vars []domain.Var) (domain.Env, error) {
	return modifyEnv(ctx, uc.repo, uc.clock, name, func(env *domain.Env) error {
		for _, v := range vars {
			if err := domain.ValidateKey(v.Key); err != nil {
				return err
			}
			env.Set(v.Key, v.Value)
		}
		return nil
	})
}

func modifyEnv(ctx context.Context, repo domain.EnvRepository, clock domain.Clock, name string, change func(*domain.Env) error) (domain.Env, error) {
	var result domain.Env
	err := repo.Update(ctx, func(s *domain.Snapshot) error {
		var err error
		result, err = s.Modify(name, clock.Now(), change)
		return err
	})
	return result, err
}
