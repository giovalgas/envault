package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/giovalgas/envault/internal/config"
	"github.com/giovalgas/envault/internal/crypto"
	"github.com/giovalgas/envault/internal/store"
	"github.com/giovalgas/envault/internal/vault"
)

func keyTestSeed(t *testing.T, ta *testApp) {
	t.Helper()
	ta.seed(t, vault.Env{Name: "api", Vars: []vault.Var{{Key: "TOKEN", Value: "segredo"}}})
}

func keyTestFileKey(t *testing.T, ta *testApp) []byte {
	t.Helper()
	key, err := store.NewFile(filepath.Join(ta.Home, config.KeyFileName)).Load(context.Background())
	if err != nil {
		t.Fatalf("ler arquivo key: %v", err)
	}
	return key
}

func keyTestKeychain(t *testing.T, ta *testApp) *store.Keyring {
	t.Helper()
	cfg, err := ta.App.Config()
	if err != nil {
		t.Fatal(err)
	}
	return keyKeyring(cfg)
}

func keyTestKeyFileExists(t *testing.T, ta *testApp) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(ta.Home, config.KeyFileName))
	return err == nil
}

func keyTestAssertVaultOpens(t *testing.T, ta *testApp) {
	t.Helper()
	snapshot, err := ta.vault(t).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	env, ok := snapshot.Envs["api"]
	if !ok || len(env.Vars) != 1 || env.Vars[0].Value != "segredo" {
		t.Fatalf("snapshot inesperado: %+v", snapshot.Envs)
	}
}

func TestKeyMigrateMovesKeyToKeychain(t *testing.T) {
	keyring.MockInit()
	ta := newTestApp(t)
	keyTestSeed(t, ta)
	original := keyTestFileKey(t, ta)

	if code := ta.runWith(newKeyCmd(ta.App), "key", "migrate"); code != ExitOK {
		t.Fatalf("code = %d (%s)", code, ta.Err.String())
	}
	if keyTestKeyFileExists(t, ta) {
		t.Fatal("arquivo key continua existindo")
	}
	stored, err := keyTestKeychain(t, ta).Load(context.Background())
	if err != nil || !bytes.Equal(stored, original) {
		t.Fatalf("keychain = %v, %v", stored, err)
	}
	if !strings.Contains(ta.Err.String(), "migrada") || strings.Contains(ta.Err.String(), store.EncodeKey(original)) {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	if ta.Out.Len() != 0 {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
	ta.App.KeyStore = KeychainKeyStore
	keyTestAssertVaultOpens(t, ta)
}

func TestKeyMigrateTwiceIsNoop(t *testing.T) {
	keyring.MockInit()
	ta := newTestApp(t)
	keyTestSeed(t, ta)
	if code := ta.runWith(newKeyCmd(ta.App), "key", "migrate"); code != ExitOK {
		t.Fatalf("primeira: code = %d (%s)", code, ta.Err.String())
	}
	if code := ta.runWith(newKeyCmd(ta.App), "key", "migrate"); code != ExitOK {
		t.Fatalf("segunda: code = %d (%s)", code, ta.Err.String())
	}
	if !strings.Contains(ta.Err.String(), "já está no keychain") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
}

func TestKeyMigrateKeychainUnavailable(t *testing.T) {
	keyring.MockInitWithError(errors.New("sem serviço de segredos"))
	t.Cleanup(keyring.MockInit)
	ta := newTestApp(t)
	keyTestSeed(t, ta)

	code := ta.runWith(newKeyCmd(ta.App), "key", "migrate")
	if code != ExitError {
		t.Fatalf("code = %d, want %d", code, ExitError)
	}
	stderr := ta.Err.String()
	if !strings.Contains(stderr, store.ErrKeyringUnavailable.Error()) || !strings.Contains(stderr, "mantido") {
		t.Fatalf("stderr = %q", stderr)
	}
	if !keyTestKeyFileExists(t, ta) {
		t.Fatal("arquivo key foi removido")
	}
	keyTestAssertVaultOpens(t, ta)
}

func TestKeyMigrateKeychainHoldsOtherKey(t *testing.T) {
	keyring.MockInit()
	ta := newTestApp(t)
	keyTestSeed(t, ta)
	other, err := crypto.NewKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := keyTestKeychain(t, ta).Save(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	if code := ta.runWith(newKeyCmd(ta.App), "key", "migrate"); code != ExitError {
		t.Fatalf("code = %d (%s)", code, ta.Err.String())
	}
	if !keyTestKeyFileExists(t, ta) {
		t.Fatal("arquivo key foi removido")
	}
}

func TestKeyMigrateFileKeyDoesNotOpenVault(t *testing.T) {
	keyring.MockInit()
	ta := newTestApp(t)
	keyTestSeed(t, ta)
	other, err := crypto.NewKey()
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(ta.Home, config.KeyFileName)
	if err := os.WriteFile(keyPath, []byte(store.EncodeKey(other)+"\n"), config.FilePerm); err != nil {
		t.Fatal(err)
	}
	if code := ta.runWith(newKeyCmd(ta.App), "key", "migrate"); code != ExitDecrypt {
		t.Fatalf("code = %d, want %d (%s)", code, ExitDecrypt, ta.Err.String())
	}
	if !keyTestKeyFileExists(t, ta) {
		t.Fatal("arquivo key foi removido")
	}
	if _, err := keyTestKeychain(t, ta).Load(context.Background()); !errors.Is(err, store.ErrNoKey) {
		t.Fatalf("keychain foi alterado: %v", err)
	}
}

func TestKeyMigrateWithoutVault(t *testing.T) {
	keyring.MockInit()
	ta := newTestApp(t)
	if code := ta.runWith(newKeyCmd(ta.App), "key", "migrate"); code != ExitNotInitialized {
		t.Fatalf("code = %d, want %d (%s)", code, ExitNotInitialized, ta.Err.String())
	}
}

func TestKeyMigrateKeyOnlyWithoutVaultFile(t *testing.T) {
	keyring.MockInit()
	ta := newTestApp(t)
	if _, err := store.NewFile(filepath.Join(ta.Home, config.KeyFileName)).Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	if code := ta.runWith(newKeyCmd(ta.App), "key", "migrate"); code != ExitOK {
		t.Fatalf("code = %d (%s)", code, ta.Err.String())
	}
	if keyTestKeyFileExists(t, ta) {
		t.Fatal("arquivo key continua existindo")
	}
}

func TestKeyMigrateUsageError(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.runWith(newKeyCmd(ta.App), "key", "migrate", "extra"); code != ExitUsage {
		t.Fatalf("code = %d, want %d", code, ExitUsage)
	}
}

func TestKeyWithoutSubcommandShowsHelp(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.runWith(newKeyCmd(ta.App), "key"); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(ta.Out.String(), "migrate") {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestKeychainKeyStoreInitUsesFileFirst(t *testing.T) {
	keyring.MockInit()
	ta := newTestApp(t)
	ta.App.KeyStore = KeychainKeyStore
	ta.initVault(t)
	if !keyTestKeyFileExists(t, ta) {
		t.Fatal("init não criou o arquivo key")
	}
	if _, err := keyTestKeychain(t, ta).Load(context.Background()); !errors.Is(err, store.ErrNoKey) {
		t.Fatalf("init gravou no keychain: %v", err)
	}
}

func TestKeychainKeyStoreInitWithoutKeychain(t *testing.T) {
	keyring.MockInitWithError(errors.New("sem keychain"))
	t.Cleanup(keyring.MockInit)
	ta := newTestApp(t)
	ta.App.KeyStore = KeychainKeyStore
	ta.initVault(t)
	if !keyTestKeyFileExists(t, ta) {
		t.Fatal("init não criou o arquivo key")
	}
}

func TestKeyAccountIsAbsoluteVaultDir(t *testing.T) {
	ta := newTestApp(t)
	cfg, err := ta.App.Config()
	if err != nil {
		t.Fatal(err)
	}
	account := keyAccount(cfg)
	if !filepath.IsAbs(account) || account != filepath.Clean(ta.Home) {
		t.Fatalf("account = %q, home = %q", account, ta.Home)
	}
}
