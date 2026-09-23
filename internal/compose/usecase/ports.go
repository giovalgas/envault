package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type EnvReader interface {
	ReadEnvs(ctx context.Context, names []string) ([]domain.Env, error)
}

type TargetFiles interface {
	Exists(path string) (bool, error)
	Write(path string, vars []domain.Var) error
	Merge(path string, vars []domain.Var) (int, error)
}

type GitignoreChecker interface {
	IsIgnored(path string) domain.GitignoreStatus
}
