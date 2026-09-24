package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type RenameEnv struct {
	repo  domain.EnvRepository
	clock domain.Clock
}

func NewRenameEnv(repo domain.EnvRepository, clock domain.Clock) *RenameEnv {
	return &RenameEnv{repo: repo, clock: clock}
}

func (uc *RenameEnv) Execute(ctx context.Context, oldName, newName string) (EnvView, error) {
	if err := domain.ValidateName(newName); err != nil {
		return EnvView{}, err
	}
	var result domain.Env
	err := uc.repo.Update(ctx, func(s *domain.Snapshot) error {
		var err error
		result, err = s.Rename(oldName, newName, uc.clock.Now())
		return err
	})
	return envView(result), err
}
