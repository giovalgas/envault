package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/giovalgas/envault/internal/vault/domain"
)

var errDraftFinished = errors.New("a sessão do editor já terminou")

type EditDraft struct {
	name     string
	isNew    bool
	session  domain.EditorSession
	store    func(ctx context.Context, edited domain.Env) (domain.Env, error)
	finished bool
}

func (d *EditDraft) Name() string {
	return d.name
}

func (d *EditDraft) IsNew() bool {
	return d.isNew
}

func (d *EditDraft) Warnings() []string {
	return d.session.Warnings()
}

func (d *EditDraft) Process(ctx context.Context) EditorProcess {
	return d.session.Process(ctx)
}

func (d *EditDraft) Review(runErr error) (EditResultView, error) {
	if d.finished {
		return EditResultView{}, errDraftFinished
	}
	result, err := d.session.Review(runErr)
	if errors.Is(err, domain.ErrEditReopen) {
		return EditResultView{}, err
	}
	d.finished = true
	if closeErr := d.session.Close(); closeErr != nil && err == nil {
		return EditResultView{}, fmt.Errorf("remover arquivo temporário: %w", closeErr)
	}
	return editResultView(result), err
}

func (d *EditDraft) Apply(ctx context.Context, view EditResultView) (EnvView, error) {
	result := editResultFromView(view)
	if !result.Changed {
		return envView(result.Env), nil
	}
	stored, err := d.store(ctx, result.Env)
	if err != nil {
		return EnvView{}, err
	}
	return envView(stored), nil
}

func (d *EditDraft) Close() error {
	d.finished = true
	return d.session.Close()
}

func applyEdit(ctx context.Context, repo domain.EnvRepository, clock domain.Clock, initial, edited domain.Env) (domain.Env, error) {
	return modifyEnv(ctx, repo, clock, initial.Name, func(env *domain.Env) error {
		if !env.SameContent(initial) {
			return domain.ErrEditConflict
		}
		env.Description = edited.Description
		env.Tags = edited.Tags
		env.Vars = edited.Vars
		return nil
	})
}

func storeNewEnv(ctx context.Context, repo domain.EnvRepository, clock domain.Clock, env domain.Env) (domain.Env, error) {
	var created domain.Env
	err := repo.Update(ctx, func(s *domain.Snapshot) error {
		var err error
		created, err = s.Create(env, clock.Now())
		return err
	})
	return created, err
}

func ensureAbsent(ctx context.Context, repo domain.EnvRepository, name string) error {
	snapshot, err := repo.Load(ctx)
	if err != nil {
		return err
	}
	if snapshot.Has(name) {
		return fmt.Errorf("%w: %q", domain.ErrEnvExists, name)
	}
	return nil
}
