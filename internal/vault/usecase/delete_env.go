package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type DeleteEnv struct {
	repo domain.EnvRepository
}

func NewDeleteEnv(repo domain.EnvRepository) *DeleteEnv {
	return &DeleteEnv{repo: repo}
}

func (uc *DeleteEnv) Execute(ctx context.Context, name string) error {
	return uc.repo.Update(ctx, func(s *domain.Snapshot) error {
		return s.Delete(name)
	})
}
