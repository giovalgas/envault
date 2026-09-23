package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/teatest"

	"github.com/giovalgas/envault/internal/shared/config"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

func TestAppFirstOpenInitializesVault(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cofre")
	paths := config.PathsIn(dir)
	repo := encryptedfile.New(paths, keystore.NewFile(paths.Key))
	initVault := vaultusecase.NewInitVault(repo)
	store := VaultStore{
		ListEnvs: vaultusecase.NewListEnvs(repo),
	}
	opts := Options{
		InitVault: func(ctx context.Context) (bool, string, error) {
			result, err := initVault.Execute(ctx)
			return result.Created, result.Location, err
		},
	}

	tm := teatest.NewTestModel(t, New(context.Background(), store, opts), teatest.WithInitialTermSize(termWidth, termHeight))
	sess := &session{t: t, tm: tm}
	sess.waitFor("cofre criado em " + dir)
	m, out := sess.finish()

	if m.statusErr {
		t.Fatalf("status não deveria indicar erro: %s", out)
	}
	if !strings.Contains(m.status, dir) {
		t.Fatalf("status = %q, esperado conter %q", m.status, dir)
	}
	if _, err := os.Stat(paths.Vault); err != nil {
		t.Fatalf("vault.enc não foi criado: %v", err)
	}
	if _, err := os.Stat(paths.Key); err != nil {
		t.Fatalf("chave não foi criada: %v", err)
	}
}
