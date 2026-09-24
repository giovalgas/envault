package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type fakeKeyFile struct {
	key       []byte
	loadErr   error
	removeErr error
	removed   bool
}

func (f *fakeKeyFile) Load(context.Context) ([]byte, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	if f.key == nil {
		return nil, domain.ErrNoKey
	}
	return bytes.Clone(f.key), nil
}

func (f *fakeKeyFile) Remove(context.Context) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	f.key = nil
	f.removed = true
	return nil
}

func (f *fakeKeyFile) Path() string {
	return "/mem/envault/key"
}

type fakeKeychain struct {
	key         []byte
	loadErr     error
	saveErr     error
	readBack    []byte
	readBackErr error
	verifyErr   error
	loadsSaved  int
}

func (k *fakeKeychain) Load(context.Context) ([]byte, error) {
	if k.loadErr != nil {
		return nil, k.loadErr
	}
	if k.key == nil {
		return nil, domain.ErrNoKey
	}
	k.loadsSaved++
	switch {
	case k.loadsSaved == 1 && k.readBackErr != nil:
		return nil, k.readBackErr
	case k.loadsSaved == 1 && k.readBack != nil:
		return bytes.Clone(k.readBack), nil
	case k.loadsSaved == 2 && k.verifyErr != nil:
		return nil, k.verifyErr
	}
	return bytes.Clone(k.key), nil
}

func (k *fakeKeychain) Create(context.Context) ([]byte, error) {
	return nil, errors.New("não usado")
}

func (k *fakeKeychain) Save(_ context.Context, key []byte) error {
	if k.saveErr != nil {
		return k.saveErr
	}
	k.key = bytes.Clone(key)
	return nil
}

func (k *fakeKeychain) Service() string { return "envault" }
func (k *fakeKeychain) Account() string { return "/mem/envault" }

type keyedRepo struct {
	memRepo
	want      []byte
	keys      domain.KeyStore
	exists    bool
	existsErr error
}

func (r *keyedRepo) Exists(context.Context) (bool, error) {
	return r.exists, r.existsErr
}

func (r *keyedRepo) Load(ctx context.Context) (*domain.Snapshot, error) {
	key, err := r.keys.Load(ctx)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(key, r.want) {
		return nil, fmt.Errorf("%w: chave errada", domain.ErrDecrypt)
	}
	return domain.NewSnapshot(baseTime), nil
}

type migration struct {
	file      *fakeKeyFile
	keychain  *fakeKeychain
	vaultKey  []byte
	exists    bool
	existsErr error
}

func newMigration() *migration {
	key := bytes.Repeat([]byte{7}, domain.KeySize)
	return &migration{file: &fakeKeyFile{key: key}, keychain: &fakeKeychain{}, vaultKey: key, exists: true}
}

func (m *migration) run() (MigrateKeyResult, error) {
	open := func(keys domain.KeyStore) domain.EnvRepository {
		return &keyedRepo{want: m.vaultKey, keys: keys, exists: m.exists, existsErr: m.existsErr}
	}
	return NewMigrateKey(m.file, m.keychain, open).Execute(context.Background())
}

func TestMigrateKeyMovesKey(t *testing.T) {
	m := newMigration()
	original := bytes.Clone(m.file.key)
	result, err := m.run()
	if err != nil || !result.Migrated {
		t.Fatalf("result = %+v, %v", result, err)
	}
	if result.Service != "envault" || result.Account != "/mem/envault" || result.File != "/mem/envault/key" {
		t.Fatalf("result = %+v", result)
	}
	if !m.file.removed || !bytes.Equal(m.keychain.key, original) {
		t.Fatalf("file removed %v, keychain %v", m.file.removed, m.keychain.key)
	}
	again, err := m.run()
	if err != nil || again.Migrated {
		t.Fatalf("second run = %+v, %v", again, err)
	}
}

func TestMigrateKeyWithoutVaultFile(t *testing.T) {
	m := newMigration()
	m.exists = false
	m.vaultKey = nil
	if _, err := m.run(); err != nil || !m.file.removed {
		t.Fatalf("err = %v, removed %v", err, m.file.removed)
	}
}

func TestMigrateKeyWithoutFile(t *testing.T) {
	m := newMigration()
	m.file.key = nil
	_, err := m.run()
	if !errors.Is(err, domain.ErrNotInitialized) || !strings.Contains(err.Error(), "/mem/envault/key") {
		t.Fatalf("no key err = %v", err)
	}
	m.keychain.loadErr = fmt.Errorf("%w: %w", domain.ErrNoKey, domain.ErrKeyringUnavailable)
	if _, err := m.run(); !errors.Is(err, domain.ErrKeyringUnavailable) || errors.Is(err, domain.ErrNotInitialized) {
		t.Fatalf("unavailable err = %v", err)
	}
	boom := errors.New("boom")
	m.keychain.loadErr = boom
	if _, err := m.run(); !errors.Is(err, boom) {
		t.Fatalf("other err = %v", err)
	}
}

func TestMigrateKeyKeepsFileOnFailure(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		step    string
		arrange func(k *fakeKeychain)
	}{
		{"gravar no keychain", func(k *fakeKeychain) { k.saveErr = boom }},
		{"ler de volta do keychain", func(k *fakeKeychain) { k.readBackErr = boom }},
		{"ler de volta do keychain", func(k *fakeKeychain) { k.readBack = bytes.Repeat([]byte{1}, domain.KeySize) }},
		{"abrir o cofre com a chave do keychain", func(k *fakeKeychain) { k.verifyErr = boom }},
	}
	for _, tc := range cases {
		m := newMigration()
		tc.arrange(m.keychain)
		_, err := m.run()
		if err == nil || !strings.Contains(err.Error(), tc.step+" falhou") || !strings.Contains(err.Error(), "mantido") {
			t.Fatalf("%s: err = %v", tc.step, err)
		}
		if m.file.removed {
			t.Fatalf("%s: arquivo removido", tc.step)
		}
	}
}

func TestMigrateKeyFileKeyDoesNotOpenVault(t *testing.T) {
	m := newMigration()
	m.vaultKey = bytes.Repeat([]byte{9}, domain.KeySize)
	_, err := m.run()
	if !errors.Is(err, domain.ErrDecrypt) || !strings.Contains(err.Error(), "nada foi migrado") {
		t.Fatalf("err = %v", err)
	}
	if m.keychain.key != nil || m.file.removed {
		t.Fatal("migração parcial")
	}
}

func TestMigrateKeyOtherFailures(t *testing.T) {
	boom := errors.New("boom")
	m := newMigration()
	m.file.loadErr = boom
	if _, err := m.run(); !errors.Is(err, boom) {
		t.Fatalf("file load err = %v", err)
	}
	m = newMigration()
	m.existsErr = boom
	if _, err := m.run(); !errors.Is(err, boom) {
		t.Fatalf("exists err = %v", err)
	}
	m = newMigration()
	m.file.removeErr = boom
	if _, err := m.run(); !errors.Is(err, boom) || !strings.Contains(err.Error(), "não pôde ser removido") {
		t.Fatalf("remove err = %v", err)
	}
}

func TestFixedKey(t *testing.T) {
	key := fixedKey(bytes.Repeat([]byte{3}, domain.KeySize))
	loaded, err := key.Load(context.Background())
	if err != nil || !bytes.Equal(loaded, key) {
		t.Fatalf("Load = %v, %v", loaded, err)
	}
	loaded[0] = 0
	if key[0] != 3 {
		t.Fatal("Load shares memory")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := key.Load(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled err = %v", err)
	}
	if _, err := key.Create(context.Background()); !errors.Is(err, domain.ErrKeyExists) {
		t.Fatalf("Create err = %v", err)
	}
}
