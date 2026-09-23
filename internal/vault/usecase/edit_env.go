package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type EditEnv struct {
	repo   domain.EnvRepository
	editor domain.Editor
	clock  domain.Clock
}

func NewEditEnv(repo domain.EnvRepository, editor domain.Editor, clock domain.Clock) *EditEnv {
	return &EditEnv{repo: repo, editor: editor, clock: clock}
}

func (uc *EditEnv) Execute(ctx context.Context, name string) (domain.EditResult, error) {
	initial, err := loadEnv(ctx, uc.repo, name)
	if err != nil {
		return domain.EditResult{}, err
	}
	result, err := uc.editor.Edit(ctx, initial, false)
	if err != nil {
		return domain.EditResult{}, err
	}
	if !result.Changed {
		return result, nil
	}
	if _, err := applyEdit(ctx, uc.repo, uc.clock, initial, result.Env); err != nil {
		return domain.EditResult{}, err
	}
	return result, nil
}
