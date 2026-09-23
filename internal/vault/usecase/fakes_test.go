package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/giovalgas/envault/internal/vault/domain"
)

var (
	baseTime = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	clock    = domain.Clock(func() time.Time { return baseTime })
)

type memRepo struct {
	snapshot    *domain.Snapshot
	initialized bool
	loadErr     error
	updates     int
}

func newRepo(envs ...domain.Env) *memRepo {
	r := &memRepo{snapshot: domain.NewSnapshot(baseTime), initialized: true}
	for _, env := range envs {
		if _, err := r.snapshot.Create(env, baseTime); err != nil {
			panic(err)
		}
	}
	return r
}

func (r *memRepo) Location() string {
	return "/mem/envault"
}

func (r *memRepo) Exists(context.Context) (bool, error) {
	return r.initialized, nil
}

func (r *memRepo) Init(context.Context) (bool, error) {
	if r.loadErr != nil {
		return false, r.loadErr
	}
	if r.initialized {
		return false, nil
	}
	r.initialized = true
	r.snapshot = domain.NewSnapshot(baseTime)
	return true, nil
}

func (r *memRepo) Load(context.Context) (*domain.Snapshot, error) {
	if !r.initialized {
		return nil, fmt.Errorf("%w: %s", domain.ErrNotInitialized, r.Location())
	}
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	return r.clone(), nil
}

func (r *memRepo) Update(ctx context.Context, apply func(*domain.Snapshot) error) error {
	work, err := r.Load(ctx)
	if err != nil {
		return err
	}
	if err := apply(work); err != nil {
		return err
	}
	if err := work.Validate(); err != nil {
		return err
	}
	r.snapshot = work
	r.updates++
	return nil
}

func (r *memRepo) clone() *domain.Snapshot {
	out := domain.NewSnapshot(r.snapshot.UpdatedAt)
	for name, env := range r.snapshot.Envs {
		out.Envs[name] = env.Clone()
	}
	return out
}

func (r *memRepo) env(t *testing.T, name string) domain.Env {
	t.Helper()
	env, err := r.snapshot.Get(name)
	if err != nil {
		t.Fatalf("Get(%s): %v", name, err)
	}
	return env
}

type fakeEditor struct {
	calls  int
	isNew  bool
	seen   domain.Env
	result func(initial domain.Env) (domain.EditResult, error)
}

func (e *fakeEditor) Edit(_ context.Context, initial domain.Env, isNew bool) (domain.EditResult, error) {
	e.calls++
	e.isNew = isNew
	e.seen = initial
	return e.result(initial)
}

func editTo(content domain.Env) *fakeEditor {
	return &fakeEditor{result: func(initial domain.Env) (domain.EditResult, error) {
		edited := initial.Clone()
		edited.Description = content.Description
		edited.Tags = content.Tags
		edited.Vars = content.Vars
		if edited.SameContent(initial) {
			return domain.EditResult{Env: initial}, nil
		}
		return domain.EditResult{Env: edited, Changed: true, Diff: domain.Compare(initial, edited)}, nil
	}}
}

func source(name, content string) EnvSource {
	return EnvSource{Name: name, Read: func() ([]byte, error) { return []byte(content), nil }}
}

func sampleEnv(name string) domain.Env {
	return domain.Env{
		Name:        name,
		Description: "Postgres local",
		Tags:        []string{"db", "local"},
		Vars:        []domain.Var{{Key: "DATABASE_URL", Value: "postgres://u:p@h/db"}, {Key: "POOL", Value: "10"}},
	}
}
