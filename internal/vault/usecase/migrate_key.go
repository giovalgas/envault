package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type KeyFile interface {
	Load(ctx context.Context) ([]byte, error)
	Remove(ctx context.Context) error
	Path() string
}

type Keychain interface {
	domain.KeyStore
	Save(ctx context.Context, key []byte) error
	Service() string
	Account() string
}

type OpenRepository func(keys domain.KeyStore) domain.EnvRepository

type MigrateKeyResult struct {
	Migrated bool
	Service  string
	Account  string
	File     string
}

type MigrateKey struct {
	file     KeyFile
	keychain Keychain
	open     OpenRepository
}

func NewMigrateKey(file KeyFile, keychain Keychain, open OpenRepository) *MigrateKey {
	return &MigrateKey{file: file, keychain: keychain, open: open}
}

func (uc *MigrateKey) Execute(ctx context.Context) (MigrateKeyResult, error) {
	key, err := uc.file.Load(ctx)
	if errors.Is(err, domain.ErrNoKey) {
		return uc.withoutFile(ctx)
	}
	if err != nil {
		return MigrateKeyResult{}, err
	}
	fileKey := fixedKey(key)
	vaultExists, err := uc.open(fileKey).Exists(ctx)
	if err != nil {
		return MigrateKeyResult{}, err
	}
	if err := uc.verify(ctx, fileKey, vaultExists); err != nil {
		return MigrateKeyResult{}, fmt.Errorf("a chave do arquivo %s não abre o cofre, nada foi migrado: %w", uc.file.Path(), err)
	}
	if err := uc.keychain.Save(ctx, key); err != nil {
		return MigrateKeyResult{}, uc.keptError("gravar no keychain", err)
	}
	readBack, err := uc.keychain.Load(ctx)
	if err != nil {
		return MigrateKeyResult{}, uc.keptError("ler de volta do keychain", err)
	}
	if !bytes.Equal(readBack, key) {
		return MigrateKeyResult{}, uc.keptError("ler de volta do keychain", errors.New("a chave lida difere da gravada"))
	}
	if err := uc.verify(ctx, uc.keychain, vaultExists); err != nil {
		return MigrateKeyResult{}, uc.keptError("abrir o cofre com a chave do keychain", err)
	}
	if err := uc.file.Remove(ctx); err != nil {
		return MigrateKeyResult{}, fmt.Errorf("chave copiada para o keychain, mas o arquivo %s não pôde ser removido: %w", uc.file.Path(), err)
	}
	return uc.result(true), nil
}

func (uc *MigrateKey) withoutFile(ctx context.Context) (MigrateKeyResult, error) {
	_, err := uc.keychain.Load(ctx)
	switch {
	case err == nil:
		return uc.result(false), nil
	case errors.Is(err, domain.ErrKeyringUnavailable):
		return MigrateKeyResult{}, err
	case errors.Is(err, domain.ErrNoKey):
		return MigrateKeyResult{}, fmt.Errorf("%w: arquivo de chave %s não existe; rode envault init", domain.ErrNotInitialized, uc.file.Path())
	default:
		return MigrateKeyResult{}, err
	}
}

func (uc *MigrateKey) verify(ctx context.Context, keys domain.KeyStore, vaultExists bool) error {
	if !vaultExists {
		return nil
	}
	_, err := uc.open(keys).Load(ctx)
	return err
}

func (uc *MigrateKey) keptError(step string, err error) error {
	return fmt.Errorf("%s falhou, o arquivo %s foi mantido: %w", step, uc.file.Path(), err)
}

func (uc *MigrateKey) result(migrated bool) MigrateKeyResult {
	return MigrateKeyResult{
		Migrated: migrated,
		Service:  uc.keychain.Service(),
		Account:  uc.keychain.Account(),
		File:     uc.file.Path(),
	}
}

type fixedKey []byte

func (k fixedKey) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return bytes.Clone(k), nil
}

func (fixedKey) Create(context.Context) ([]byte, error) {
	return nil, fmt.Errorf("%w: chave fixa da migração", domain.ErrKeyExists)
}
