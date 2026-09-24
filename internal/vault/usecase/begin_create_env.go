package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type BeginCreateEnv struct {
	repo   domain.EnvRepository
	editor domain.SessionEditor
	clock  domain.Clock
}

func NewBeginCreateEnv(repo domain.EnvRepository, editor domain.SessionEditor, clock domain.Clock) *BeginCreateEnv {
	return &BeginCreateEnv{repo: repo, editor: editor, clock: clock}
}

func (uc *BeginCreateEnv) Execute(ctx context.Context, name string) (*EditDraft, error) {
	env := domain.Env{Name: name}
	if err := env.Validate(); err != nil {
		return nil, err
	}
	if err := ensureAbsent(ctx, uc.repo, env.Name); err != nil {
		return nil, err
	}
	session, err := uc.editor.Open(env, true)
	if err != nil {
		return nil, err
	}
	return &EditDraft{
		name:    env.Name,
		isNew:   true,
		session: session,
		store: func(ctx context.Context, edited domain.Env) (domain.Env, error) {
			return storeNewEnv(ctx, uc.repo, uc.clock, edited)
		},
	}, nil
}
