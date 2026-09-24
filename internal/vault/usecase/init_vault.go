package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type InitVaultResult struct {
	Created  bool
	Location string
}

type InitVault struct {
	repo domain.EnvRepository
}

func NewInitVault(repo domain.EnvRepository) *InitVault {
	return &InitVault{repo: repo}
}

func (uc *InitVault) Execute(ctx context.Context) (InitVaultResult, error) {
	created, err := uc.repo.Init(ctx)
	if err != nil {
		return InitVaultResult{}, err
	}
	return InitVaultResult{Created: created, Location: uc.repo.Location()}, nil
}
