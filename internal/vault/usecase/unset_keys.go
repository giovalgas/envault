package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type UnsetKeys struct {
	repo  domain.EnvRepository
	clock domain.Clock
}

func NewUnsetKeys(repo domain.EnvRepository, clock domain.Clock) *UnsetKeys {
	return &UnsetKeys{repo: repo, clock: clock}
}

func (uc *UnsetKeys) Execute(ctx context.Context, name string, keys []string) ([]string, error) {
	var removed []string
	_, err := modifyEnv(ctx, uc.repo, uc.clock, name, func(env *domain.Env) error {
		removed = env.Unset(keys...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return removed, nil
}
