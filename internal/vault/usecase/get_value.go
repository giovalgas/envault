package usecase

import (
	"context"
	"fmt"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type GetValue struct {
	repo domain.EnvRepository
}

func NewGetValue(repo domain.EnvRepository) *GetValue {
	return &GetValue{repo: repo}
}

func (uc *GetValue) Execute(ctx context.Context, name, key string) (string, error) {
	env, err := loadEnv(ctx, uc.repo, name)
	if err != nil {
		return "", err
	}
	value, ok := env.Lookup(key)
	if !ok {
		return "", fmt.Errorf("%w: %q em %q", domain.ErrVarNotFound, key, name)
	}
	return value, nil
}
