package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type BeginEditEnv struct {
	repo   domain.EnvRepository
	editor domain.SessionEditor
	clock  domain.Clock
}

func NewBeginEditEnv(repo domain.EnvRepository, editor domain.SessionEditor, clock domain.Clock) *BeginEditEnv {
	return &BeginEditEnv{repo: repo, editor: editor, clock: clock}
}

func (uc *BeginEditEnv) Execute(ctx context.Context, name string) (*EditDraft, error) {
	initial, err := loadEnv(ctx, uc.repo, name)
	if err != nil {
		return nil, err
	}
	session, err := uc.editor.Open(initial, false)
	if err != nil {
		return nil, err
	}
	return &EditDraft{
		name:    initial.Name,
		session: session,
		store: func(ctx context.Context, edited domain.Env) (domain.Env, error) {
			return applyEdit(ctx, uc.repo, uc.clock, initial, edited)
		},
	}, nil
}
