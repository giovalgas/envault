package keystore

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/giovalgas/envault/internal/vault/domain"
)

const keyringTestAccount = "/tmp/envault-test"

func newMockKeyring(t *testing.T) *Keyring {
	t.Helper()
	keyring.MockInit()
	return NewKeyring(keyringTestAccount)
}

func newKeyringTestKey(t *testing.T) []byte {
	t.Helper()
	key, err := domain.NewKey()
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	return key
}

func TestKeyringIdentity(t *testing.T) {
	k := NewKeyring("conta")
	if k.Service() != "envault" || k.Account() != "conta" {
		t.Fatalf("service/account = %q/%q", k.Service(), k.Account())
	}
}

func TestKeyringLoadMissing(t *testing.T) {
	k := newMockKeyring(t)
	if _, err := k.Load(context.Background()); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("err = %v, want domain.ErrNoKey", err)
	}
}

func TestKeyringCreateThenLoad(t *testing.T) {
	k := newMockKeyring(t)
	created, err := k.Create(context.Background())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	loaded, err := k.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(created, loaded) || len(loaded) != domain.KeySize {
		t.Fatal("chave lida difere da criada")
	}
	if _, err := k.Create(context.Background()); !errors.Is(err, domain.ErrKeyExists) {
		t.Fatalf("segundo Create err = %v, want domain.ErrKeyExists", err)
	}
}

func TestKeyringSaveIdempotentAndConflict(t *testing.T) {
	k := newMockKeyring(t)
	key := newKeyringTestKey(t)
	if err := k.Save(context.Background(), key); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := k.Save(context.Background(), key); err != nil {
		t.Fatalf("Save repetido: %v", err)
	}
	if err := k.Save(context.Background(), newKeyringTestKey(t)); !errors.Is(err, domain.ErrKeyExists) {
		t.Fatalf("Save de outra chave err = %v, want domain.ErrKeyExists", err)
	}
	got, err := k.Load(context.Background())
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("Load = %v, %v", got, err)
	}
}

func TestKeyringSaveRejectsMalformed(t *testing.T) {
	k := newMockKeyring(t)
	if err := k.Save(context.Background(), []byte("curta")); !errors.Is(err, domain.ErrMalformedKey) {
		t.Fatalf("err = %v, want domain.ErrMalformedKey", err)
	}
}

func TestKeyringLoadMalformed(t *testing.T) {
	k := newMockKeyring(t)
	if err := keyring.Set(KeyringService, keyringTestAccount, "não é base64"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Load(context.Background()); !errors.Is(err, domain.ErrMalformedKey) {
		t.Fatalf("err = %v, want domain.ErrMalformedKey", err)
	}
}

func TestKeyringAccountsAreIsolated(t *testing.T) {
	keyring.MockInit()
	a, b := NewKeyring("/a"), NewKeyring("/b")
	if _, err := a.Create(context.Background()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := b.Load(context.Background()); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("conta b err = %v, want domain.ErrNoKey", err)
	}
}

func TestKeyringRemove(t *testing.T) {
	k := newMockKeyring(t)
	if err := k.Remove(context.Background()); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("Remove vazio err = %v, want domain.ErrNoKey", err)
	}
	if _, err := k.Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := k.Remove(context.Background()); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := k.Load(context.Background()); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("Load após Remove err = %v", err)
	}
}

func TestKeyringUnavailable(t *testing.T) {
	boom := errors.New("sem dbus")
	keyring.MockInitWithError(boom)
	t.Cleanup(keyring.MockInit)
	k := NewKeyring(keyringTestAccount)

	_, err := k.Load(context.Background())
	if !errors.Is(err, domain.ErrNoKey) || !errors.Is(err, domain.ErrKeyringUnavailable) || !errors.Is(err, boom) {
		t.Fatalf("Load err = %v, want domain.ErrNoKey + domain.ErrKeyringUnavailable", err)
	}
	if err := k.Save(context.Background(), newKeyringTestKey(t)); !errors.Is(err, domain.ErrKeyringUnavailable) || errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("Save err = %v, want domain.ErrKeyringUnavailable", err)
	}
	if _, err := k.Create(context.Background()); !errors.Is(err, domain.ErrKeyringUnavailable) {
		t.Fatalf("Create err = %v", err)
	}
	if err := k.Remove(context.Background()); !errors.Is(err, domain.ErrKeyringUnavailable) {
		t.Fatalf("Remove err = %v", err)
	}
}

func TestKeyringChainFallsBackWhenUnavailable(t *testing.T) {
	keyring.MockInitWithError(errors.New("sem keychain"))
	t.Cleanup(keyring.MockInit)
	fallback := &fakeStore{key: newKeyringTestKey(t)}
	chain := Chain{&fakeStore{loadErr: domain.ErrNoKey}, NewKeyring(keyringTestAccount), fallback}
	got, err := chain.Load(context.Background())
	if err != nil || !bytes.Equal(got, fallback.key) {
		t.Fatalf("Load = %v, %v", got, err)
	}
}

func TestKeyringChainFileFirst(t *testing.T) {
	k := newMockKeyring(t)
	inKeyring, err := k.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	file := NewFile(filepath.Join(t.TempDir(), "key"))
	chain := Chain{file, k}
	got, err := chain.Load(context.Background())
	if err != nil || !bytes.Equal(got, inKeyring) {
		t.Fatalf("sem arquivo, Load = %v, %v", got, err)
	}
	inFile, err := file.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err = chain.Load(context.Background())
	if err != nil || !bytes.Equal(got, inFile) {
		t.Fatalf("com arquivo, Load = %v, %v", got, err)
	}
}

func TestKeyringCanceledContext(t *testing.T) {
	k := newMockKeyring(t)
	ctx := canceledContext()
	if _, err := k.Load(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load err = %v", err)
	}
	if err := k.Save(ctx, newKeyringTestKey(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save err = %v", err)
	}
	if err := k.Remove(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Remove err = %v", err)
	}
}

func TestAccountIsAbsoluteVaultDir(t *testing.T) {
	home := filepath.Join(t.TempDir(), "envault")
	if account := Account(home); !filepath.IsAbs(account) || account != filepath.Clean(home) {
		t.Fatalf("account = %q, home = %q", account, home)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if account := Account("envault"); account != filepath.Join(wd, "envault") {
		t.Fatalf("relative account = %q", account)
	}
}
