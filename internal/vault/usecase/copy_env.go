package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type CopyEnv struct {
	repo  domain.EnvRepository
	clock domain.Clock
}

func NewCopyEnv(repo domain.EnvRepository, clock domain.Clock) *CopyEnv {
	return &CopyEnv{repo: repo, clock: clock}
}

func (uc *CopyEnv) Execute(ctx context.Context, src, dst string) (EnvView, error) {
	if err := domain.ValidateName(dst); err != nil {
		return EnvView{}, err
	}
	var result domain.Env
	err := uc.repo.Update(ctx, func(s *domain.Snapshot) error {
		var err error
		result, err = s.Copy(src, dst, uc.clock.Now())
		return err
	})
	return envView(result), err
}
