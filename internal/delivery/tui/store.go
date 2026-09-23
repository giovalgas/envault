package tui

import (
	"context"

	vault "github.com/giovalgas/envault/internal/vault/domain"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

type VaultStore struct {
	ListEnvs  *vaultusecase.ListEnvs
	ShowEnv   *vaultusecase.ShowEnv
	CopyEnv   *vaultusecase.CopyEnv
	RenameEnv *vaultusecase.RenameEnv
	DeleteEnv *vaultusecase.DeleteEnv
}

var _ Store = VaultStore{}

func (s VaultStore) List(ctx context.Context) ([]vault.Env, error) {
	return s.ListEnvs.Execute(ctx, vaultusecase.ListEnvsQuery{})
}

func (s VaultStore) Get(ctx context.Context, name string) (vault.Env, error) {
	return s.ShowEnv.Execute(ctx, name)
}

func (s VaultStore) Copy(ctx context.Context, src, dst string) (vault.Env, error) {
	return s.CopyEnv.Execute(ctx, src, dst)
}

func (s VaultStore) Rename(ctx context.Context, oldName, newName string) (vault.Env, error) {
	return s.RenameEnv.Execute(ctx, oldName, newName)
}

func (s VaultStore) Delete(ctx context.Context, name string) error {
	return s.DeleteEnv.Execute(ctx, name)
}
