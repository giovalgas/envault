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
	_, err = modifyEnv(ctx, uc.repo, uc.clock, name, func(env *domain.Env) error {
		if !env.SameContent(initial) {
			return domain.ErrEditConflict
		}
		env.Description = result.Env.Description
		env.Tags = result.Env.Tags
		env.Vars = result.Env.Vars
		return nil
	})
	if err != nil {
		return domain.EditResult{}, err
	}
	return result, nil
}
