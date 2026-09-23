package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type ShowEnv struct {
	repo domain.EnvRepository
}

func NewShowEnv(repo domain.EnvRepository) *ShowEnv {
	return &ShowEnv{repo: repo}
}

func (uc *ShowEnv) Execute(ctx context.Context, name string) (domain.Env, error) {
	return loadEnv(ctx, uc.repo, name)
}

func loadEnv(ctx context.Context, repo domain.EnvRepository, name string) (domain.Env, error) {
	snapshot, err := repo.Load(ctx)
	if err != nil {
		return domain.Env{}, err
	}
	return snapshot.Get(name)
}
