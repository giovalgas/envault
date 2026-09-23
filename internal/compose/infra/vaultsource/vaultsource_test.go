package vaultsource

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/giovalgas/envault/internal/compose/domain"
	"github.com/giovalgas/envault/internal/compose/usecase"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

var errBoom = errors.New("boom")

type memRepo struct {
	snapshot *vault.Snapshot
	loadErr  error
	loads    int
}

func newRepo(t *testing.T, envs ...vault.Env) *memRepo {
	t.Helper()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	repo := &memRepo{snapshot: vault.NewSnapshot(now)}
	for _, env := range envs {
		if _, err := repo.snapshot.Create(env, now); err != nil {
			t.Fatalf("Create(%s): %v", env.Name, err)
		}
	}
	return repo
}

func (r *memRepo) Location() string                     { return "/mem" }
func (r *memRepo) Exists(context.Context) (bool, error) { return true, nil }
func (r *memRepo) Init(context.Context) (bool, error)   { return false, nil }

func (r *memRepo) Load(context.Context) (*vault.Snapshot, error) {
	r.loads++
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	return r.snapshot, nil
}

func (r *memRepo) Update(context.Context, func(*vault.Snapshot) error) error {
	return errBoom
}

func openRepo(repo *memRepo) OpenListEnvs {
	return func() (*vaultusecase.ListEnvs, error) { return vaultusecase.NewListEnvs(repo), nil }
}

func TestLoadEnvsFromVaultInRequestedOrder(t *testing.T) {
	repo := newRepo(t,
		vault.Env{Name: "a", Description: "d", Tags: []string{"x"}, Vars: []vault.Var{{Key: "X", Value: "1"}, {Key: "Y", Value: "2"}}},
		vault.Env{Name: "b", Vars: []vault.Var{{Key: "X", Value: "3"}}},
		vault.Env{Name: "c"},
	)
	envs, err := New(openRepo(repo)).ReadEnvs(context.Background(), []string{"b", "a", "c"})
	if err != nil {
		t.Fatalf("ReadEnvs: %v", err)
	}
	want := []domain.Env{
		{Name: "b", Vars: []domain.Var{{Key: "X", Value: "3"}}},
		{Name: "a", Vars: []domain.Var{{Key: "X", Value: "1"}, {Key: "Y", Value: "2"}}},
		{Name: "c", Vars: []domain.Var{}},
	}
	if !reflect.DeepEqual(envs, want) {
		t.Fatalf("envs = %+v", envs)
	}
	if repo.loads != 1 {
		t.Fatalf("loads = %d, want 1", repo.loads)
	}
}

func TestLoadEnvsFromVaultMissing(t *testing.T) {
	repo := newRepo(t, vault.Env{Name: "a"})
	_, err := New(openRepo(repo)).ReadEnvs(context.Background(), []string{"a", "x", "y"})
	var missing *usecase.EnvNotFoundError
	if !errors.As(err, &missing) || missing.Name != "x" {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadEnvsFromVaultErrors(t *testing.T) {
	failing := New(func() (*vaultusecase.ListEnvs, error) { return nil, errBoom })
	if _, err := failing.ReadEnvs(context.Background(), []string{"a"}); !errors.Is(err, errBoom) {
		t.Fatalf("open err = %v", err)
	}
	repo := newRepo(t)
	repo.loadErr = vault.ErrNotInitialized
	if _, err := New(openRepo(repo)).ReadEnvs(context.Background(), []string{"a"}); !errors.Is(err, vault.ErrNotInitialized) {
		t.Fatalf("load err = %v", err)
	}
}
