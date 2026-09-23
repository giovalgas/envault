package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"

	"github.com/giovalgas/envault/internal/config"
	"github.com/giovalgas/envault/internal/crypto"
	"github.com/giovalgas/envault/internal/store"
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type brokenKeys struct {
	loadErr   error
	createErr error
}

func (b brokenKeys) Load(context.Context) ([]byte, error)   { return nil, b.loadErr }
func (b brokenKeys) Create(context.Context) ([]byte, error) { return nil, b.createErr }

type fixture struct {
	vault *Vault
	paths config.Paths
	clock *clock
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	paths := config.PathsIn(filepath.Join(t.TempDir(), "envault"))
	c := &clock{now: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)}
	return &fixture{
		vault: New(paths, store.NewFile(paths.Key), WithClock(c.Now)),
		paths: paths,
		clock: c,
	}
}

func newInitialized(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	if _, err := f.vault.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return f
}

func (f *fixture) reopen() *Vault {
	return New(f.paths, store.NewFile(f.paths.Key), WithClock(f.clock.Now))
}

func sampleEnv(name string) Env {
	return Env{
		Name:        name,
		Description: "Postgres local",
		Tags:        []string{"db", "local"},
		Vars: []Var{
			{Key: "B", Value: "2"},
			{Key: "A", Value: "1"},
			{Key: "C", Value: "3"},
		},
	}
}

func mustCreate(t *testing.T, v *Vault, env Env) Env {
	t.Helper()
	created, err := v.Create(context.Background(), env)
	if err != nil {
		t.Fatalf("Create(%s): %v", env.Name, err)
	}
	return created
}

func readVaultFile(t *testing.T, f *fixture) []byte {
	t.Helper()
	data, err := os.ReadFile(f.paths.Vault)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return data
}

func writeSealed(t *testing.T, f *fixture, plaintext string) {
	t.Helper()
	key, err := store.NewFile(f.paths.Key).Load(context.Background())
	if err != nil {
		t.Fatalf("Load key: %v", err)
	}
	sealed, err := crypto.Seal(key, []byte(plaintext))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if err := os.WriteFile(f.paths.Vault, sealed, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestInitCreatesFiles(t *testing.T) {
	f := newFixture(t)
	created, err := f.vault.Init(context.Background())
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !created {
		t.Fatal("created = false on first Init")
	}
	for _, path := range []string{f.paths.Vault, f.paths.Key} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%s): %v", path, err)
		}
	}
	envs, err := f.vault.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(envs) != 0 {
		t.Fatalf("len(envs) = %d", len(envs))
	}
	if f.vault.Paths() != f.paths {
		t.Fatal("Paths mismatch")
	}
}

func TestInitPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissões Unix não se aplicam no Windows")
	}
	f := newInitialized(t)
	mustCreate(t, f.vault, sampleEnv("a"))
	checks := map[string]os.FileMode{f.paths.Dir: 0o700, f.paths.Vault: 0o600, f.paths.Key: 0o600}
	for path, want := range checks {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%s): %v", path, err)
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s perm = %o, want %o", path, info.Mode().Perm(), want)
		}
	}
}

func TestInitIdempotent(t *testing.T) {
	f := newInitialized(t)
	mustCreate(t, f.vault, sampleEnv("a"))
	before := readVaultFile(t, f)
	created, err := f.reopen().Init(context.Background())
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if created {
		t.Fatal("created = true on second Init")
	}
	if !bytes.Equal(before, readVaultFile(t, f)) {
		t.Fatal("second Init changed vault.enc")
	}
}

func TestInitWithExistingVaultWithoutKey(t *testing.T) {
	f := newInitialized(t)
	if err := os.Remove(f.paths.Key); err != nil {
		t.Fatalf("Remove key: %v", err)
	}
	_, err := f.reopen().Init(context.Background())
	if !errors.Is(err, ErrDecrypt) || !errors.Is(err, store.ErrNoKey) {
		t.Fatalf("err = %v, want ErrDecrypt and ErrNoKey", err)
	}
	if _, err := os.Stat(f.paths.Key); err == nil {
		t.Fatal("Init recreated the key over an existing vault")
	}
}

func TestInitWithExistingKeyWithoutVault(t *testing.T) {
	f := newFixture(t)
	key, err := store.NewFile(f.paths.Key).Create(context.Background())
	if err != nil {
		t.Fatalf("Create key: %v", err)
	}
	if _, err := f.vault.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	loaded, err := store.NewFile(f.paths.Key).Load(context.Background())
	if err != nil || !bytes.Equal(key, loaded) {
		t.Fatalf("key changed: %v", err)
	}
}

func TestInitErrors(t *testing.T) {
	boom := errors.New("boom")
	paths := config.PathsIn(filepath.Join(t.TempDir(), "envault"))
	if _, err := New(paths, brokenKeys{loadErr: store.ErrNoKey, createErr: boom}).Init(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("create failure err = %v", err)
	}
	if _, err := New(paths, brokenKeys{loadErr: boom}).Init(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("load failure err = %v", err)
	}

	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	blocked := config.PathsIn(filepath.Join(blocker, "envault"))
	if _, err := New(blocked, store.NewFile(blocked.Key)).Init(context.Background()); err == nil {
		t.Fatal("Init succeeded under a regular file")
	}
}

func TestInitOverWrongKeyFails(t *testing.T) {
	f := newInitialized(t)
	other, err := store.NewStatic(bytes.Repeat([]byte{5}, crypto.KeySize))
	if err != nil {
		t.Fatalf("NewStatic: %v", err)
	}
	if _, err := New(f.paths, other).Init(context.Background()); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestOperationsRequireInit(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	checks := map[string]error{}
	_, checks["list"] = f.vault.List(ctx)
	_, checks["get"] = f.vault.Get(ctx, "a")
	_, checks["create"] = f.vault.Create(ctx, sampleEnv("a"))
	checks["delete"] = f.vault.Delete(ctx, "a")
	checks["update"] = f.vault.Update(ctx, func(*Snapshot) error { return nil })
	for op, err := range checks {
		if !errors.Is(err, ErrNotInitialized) {
			t.Errorf("%s err = %v, want ErrNotInitialized", op, err)
		}
	}
	if _, err := os.Stat(f.paths.Dir); err == nil {
		t.Fatal("operations without init created the vault dir")
	}
	exists, err := f.vault.Exists()
	if err != nil || exists {
		t.Fatalf("Exists = %v, %v", exists, err)
	}
}

func TestCreateGetPreservesOrder(t *testing.T) {
	f := newInitialized(t)
	created := mustCreate(t, f.vault, sampleEnv("a"))
	if !created.CreatedAt.Equal(f.clock.Now()) || !created.UpdatedAt.Equal(f.clock.Now()) {
		t.Fatalf("timestamps = %v, %v", created.CreatedAt, created.UpdatedAt)
	}
	env, err := f.reopen().Get(context.Background(), "a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := strings.Join(env.Keys(), ","); got != "B,A,C" {
		t.Fatalf("keys = %s, want B,A,C", got)
	}
	if env.Name != "a" || !env.SameContent(sampleEnv("a")) {
		t.Fatalf("env = %+v", env)
	}
}

func TestCreateValidation(t *testing.T) {
	f := newInitialized(t)
	before := readVaultFile(t, f)
	cases := []struct {
		name string
		env  Env
		want error
		cite string
	}{
		{"nome inválido", Env{Name: "Postgres Local"}, ErrInvalidName, "Postgres Local"},
		{"chave duplicada", Env{Name: "a", Vars: []Var{{Key: "A", Value: "1"}, {Key: "A", Value: "2"}}}, ErrInvalidKey, "A"},
		{"chave inválida", Env{Name: "a", Vars: []Var{{Key: "1KEY", Value: "x"}}}, ErrInvalidKey, "1KEY"},
		{"tag com espaço", Env{Name: "a", Tags: []string{"Local DB"}}, ErrInvalidTag, "Local DB"},
		{"descrição multilinha", Env{Name: "a", Description: "a\nb"}, ErrInvalidDescription, "quebra"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.vault.Create(context.Background(), tc.env)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.cite) {
				t.Fatalf("err %q does not cite %q", err, tc.cite)
			}
		})
	}
	if !bytes.Equal(before, readVaultFile(t, f)) {
		t.Fatal("invalid operations changed the vault")
	}
}

func TestErrorsDoNotLeakValues(t *testing.T) {
	f := newInitialized(t)
	_, err := f.vault.Create(context.Background(), Env{Name: "a", Vars: []Var{{Key: "A", Value: "supersegredo"}, {Key: "A", Value: "supersegredo"}}})
	if err == nil || strings.Contains(err.Error(), "supersegredo") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateDuplicate(t *testing.T) {
	f := newInitialized(t)
	mustCreate(t, f.vault, sampleEnv("a"))
	if _, err := f.vault.Create(context.Background(), sampleEnv("a")); !errors.Is(err, ErrEnvExists) {
		t.Fatalf("err = %v, want ErrEnvExists", err)
	}
}

func TestGetMissing(t *testing.T) {
	f := newInitialized(t)
	_, err := f.vault.Get(context.Background(), "nao-existe")
	if !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("err = %v, want ErrEnvNotFound", err)
	}
	if !strings.Contains(err.Error(), "nao-existe") {
		t.Fatalf("err %q does not cite name", err)
	}
}

func TestReplaceAndPut(t *testing.T) {
	f := newInitialized(t)
	ctx := context.Background()
	replacement := Env{Name: "a", Vars: []Var{{Key: "X", Value: "9"}}}
	if _, err := f.vault.Replace(ctx, replacement); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("Replace missing err = %v", err)
	}
	original := mustCreate(t, f.vault, sampleEnv("a"))
	f.clock.Advance(time.Hour)
	replaced, err := f.vault.Replace(ctx, replacement)
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if !replaced.CreatedAt.Equal(original.CreatedAt) {
		t.Fatal("Replace changed created_at")
	}
	if !replaced.UpdatedAt.Equal(f.clock.Now()) {
		t.Fatal("Replace did not bump updated_at")
	}
	got, err := f.vault.Get(ctx, "a")
	if err != nil || !got.SameContent(replacement) {
		t.Fatalf("Get = %+v, %v", got, err)
	}

	put, err := f.vault.Put(ctx, Env{Name: "b", Vars: []Var{{Key: "Y", Value: "1"}}})
	if err != nil {
		t.Fatalf("Put new: %v", err)
	}
	f.clock.Advance(time.Hour)
	again, err := f.vault.Put(ctx, Env{Name: "b", Vars: []Var{{Key: "Z", Value: "2"}}})
	if err != nil {
		t.Fatalf("Put existing: %v", err)
	}
	if !again.CreatedAt.Equal(put.CreatedAt) || strings.Join(again.Keys(), ",") != "Z" {
		t.Fatalf("Put existing = %+v", again)
	}
}

func TestModify(t *testing.T) {
	f := newInitialized(t)
	ctx := context.Background()
	mustCreate(t, f.vault, sampleEnv("a"))
	f.clock.Advance(time.Minute)
	env, err := f.vault.Modify(ctx, "a", func(e *Env) error {
		e.Set("A", "10")
		e.Set("D", "4")
		e.Unset("B")
		return nil
	})
	if err != nil {
		t.Fatalf("Modify: %v", err)
	}
	if got := strings.Join(env.Keys(), ","); got != "A,C,D" {
		t.Fatalf("keys = %s", got)
	}
	if value, _ := env.Lookup("A"); value != "10" {
		t.Fatalf("A = %q", value)
	}
	if !env.UpdatedAt.Equal(f.clock.Now()) {
		t.Fatal("Modify did not bump updated_at")
	}
	if _, err := f.vault.Modify(ctx, "x", func(*Env) error { return nil }); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	boom := errors.New("boom")
	if _, err := f.vault.Modify(ctx, "a", func(*Env) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("change err = %v", err)
	}
	if _, err := f.vault.Modify(ctx, "a", func(e *Env) error { e.Set("1BAD", "x"); return nil }); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("invalid key err = %v", err)
	}
}

func TestRename(t *testing.T) {
	f := newInitialized(t)
	ctx := context.Background()
	mustCreate(t, f.vault, sampleEnv("a"))
	mustCreate(t, f.vault, sampleEnv("b"))
	if _, err := f.vault.Rename(ctx, "a", "b"); !errors.Is(err, ErrEnvExists) {
		t.Fatalf("occupied err = %v, want ErrEnvExists", err)
	}
	for _, name := range []string{"a", "b"} {
		if _, err := f.vault.Get(ctx, name); err != nil {
			t.Fatalf("env %s lost after failed rename: %v", name, err)
		}
	}
	renamed, err := f.vault.Rename(ctx, "a", "c")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.Name != "c" {
		t.Fatalf("Name = %q", renamed.Name)
	}
	if _, err := f.vault.Get(ctx, "a"); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("old name still exists: %v", err)
	}
	same, err := f.vault.Rename(ctx, "c", "c")
	if err != nil || same.Name != "c" {
		t.Fatalf("same-name rename = %+v, %v", same, err)
	}
	if _, err := f.vault.Rename(ctx, "nope", "d"); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	if _, err := f.vault.Rename(ctx, "c", "Bad Name"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("invalid err = %v", err)
	}
}

func TestCopy(t *testing.T) {
	f := newInitialized(t)
	ctx := context.Background()
	original := mustCreate(t, f.vault, sampleEnv("a"))
	f.clock.Advance(time.Hour)
	copied, err := f.vault.Copy(ctx, "a", "c")
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if !copied.SameContent(original) {
		t.Fatal("copy content differs")
	}
	if !copied.CreatedAt.After(original.CreatedAt) {
		t.Fatal("copy did not get a new created_at")
	}
	if _, err := f.vault.Copy(ctx, "a", "c"); !errors.Is(err, ErrEnvExists) {
		t.Fatalf("dst exists err = %v", err)
	}
	if _, err := f.vault.Copy(ctx, "x", "y"); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("missing src err = %v", err)
	}
	if _, err := f.vault.Copy(ctx, "a", "Y Y"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("invalid dst err = %v", err)
	}
	copied.Vars[0].Value = "mutated"
	stored, err := f.vault.Get(ctx, "a")
	if err != nil || stored.Vars[0].Value == "mutated" {
		t.Fatal("copy shares memory with source")
	}
}

func TestDelete(t *testing.T) {
	f := newInitialized(t)
	ctx := context.Background()
	mustCreate(t, f.vault, sampleEnv("a"))
	if err := f.vault.Delete(ctx, "a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := f.vault.Get(ctx, "a"); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("Get after delete err = %v", err)
	}
	if err := f.vault.Delete(ctx, "a"); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("Delete missing err = %v", err)
	}
}

func TestListSortedAndSnapshot(t *testing.T) {
	f := newInitialized(t)
	for _, name := range []string{"stripe-test", "aws-dev", "postgres-local"} {
		mustCreate(t, f.vault, sampleEnv(name))
	}
	envs, err := f.vault.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var names []string
	for _, env := range envs {
		names = append(names, env.Name)
	}
	if got := strings.Join(names, ","); got != "aws-dev,postgres-local,stripe-test" {
		t.Fatalf("names = %s", got)
	}
	snapshot, err := f.vault.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snapshot.SchemaVersion != SchemaVersion || !snapshot.UpdatedAt.Equal(f.clock.Now()) {
		t.Fatalf("snapshot header = %d %v", snapshot.SchemaVersion, snapshot.UpdatedAt)
	}
}

func TestUpdateValidatesAndPropagates(t *testing.T) {
	f := newInitialized(t)
	ctx := context.Background()
	boom := errors.New("boom")
	if err := f.vault.Update(ctx, func(*Snapshot) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	err := f.vault.Update(ctx, func(s *Snapshot) error {
		s.Envs["Bad Name"] = Env{}
		return nil
	})
	if !errors.Is(err, ErrInvalidName) {
		t.Fatalf("err = %v, want ErrInvalidName", err)
	}
	err = f.vault.Update(ctx, func(s *Snapshot) error {
		s.Envs["ok"] = Env{Name: "ignored", Vars: []Var{{Key: "K", Value: "v"}}}
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	env, err := f.vault.Get(ctx, "ok")
	if err != nil || env.Name != "ok" {
		t.Fatalf("Get = %+v, %v", env, err)
	}
}

func TestWrongKeyFails(t *testing.T) {
	f := newInitialized(t)
	mustCreate(t, f.vault, sampleEnv("a"))
	other, err := store.NewStatic(bytes.Repeat([]byte{4}, crypto.KeySize))
	if err != nil {
		t.Fatalf("NewStatic: %v", err)
	}
	if _, err := New(f.paths, other).List(context.Background()); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestStaticKeyOpensVault(t *testing.T) {
	paths := config.PathsIn(filepath.Join(t.TempDir(), "envault"))
	key := bytes.Repeat([]byte{8}, crypto.KeySize)
	ks, err := store.WithOverride(store.EncodeKey(key), store.NewFile(paths.Key))
	if err != nil {
		t.Fatalf("WithOverride: %v", err)
	}
	v := New(paths, ks)
	if _, err := v.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	mustCreate(t, v, sampleEnv("a"))
	if _, err := os.Stat(paths.Key); err == nil {
		t.Fatal("key file created despite ENVAULT_KEY")
	}
	if _, err := v.Get(context.Background(), "a"); err != nil {
		t.Fatalf("Get: %v", err)
	}
}

func TestCorruptedVault(t *testing.T) {
	f := newInitialized(t)
	if err := os.WriteFile(f.paths.Vault, []byte("lixo"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := f.vault.List(context.Background()); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestMissingKeyWithVault(t *testing.T) {
	f := newInitialized(t)
	if err := os.Remove(f.paths.Key); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_, err := f.vault.List(context.Background())
	if !errors.Is(err, ErrDecrypt) || !errors.Is(err, store.ErrNoKey) {
		t.Fatalf("err = %v", err)
	}
}

func TestMalformedKeyIsNotDecrypt(t *testing.T) {
	f := newInitialized(t)
	if err := os.WriteFile(f.paths.Key, []byte("curta"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := f.vault.List(context.Background())
	if !errors.Is(err, store.ErrMalformedKey) || errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrMalformedKey only", err)
	}
}

func TestSchemaVersions(t *testing.T) {
	cases := []struct {
		name      string
		plaintext string
		want      error
		cite      string
	}{
		{"versão futura", `{"schema_version":2,"envs":{}}`, ErrUnsupportedSchema, "2"},
		{"sem versão", `{"envs":{}}`, ErrDecrypt, "schema_version"},
		{"json inválido", `{`, ErrDecrypt, "corrompido"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newInitialized(t)
			writeSealed(t, f, tc.plaintext)
			_, err := f.vault.List(context.Background())
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.cite) {
				t.Fatalf("err %q does not cite %q", err, tc.cite)
			}
		})
	}
}

func TestNullEnvsAccepted(t *testing.T) {
	f := newInitialized(t)
	writeSealed(t, f, `{"schema_version":1,"envs":null}`)
	envs, err := f.vault.List(context.Background())
	if err != nil || len(envs) != 0 {
		t.Fatalf("List = %v, %v", envs, err)
	}
	mustCreate(t, f.vault, sampleEnv("a"))
}

func TestStoredJSONShape(t *testing.T) {
	f := newInitialized(t)
	mustCreate(t, f.vault, Env{Name: "vazia"})
	key, err := store.NewFile(f.paths.Key).Load(context.Background())
	if err != nil {
		t.Fatalf("Load key: %v", err)
	}
	plaintext, err := crypto.Open(key, readVaultFile(t, f))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(plaintext, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if raw["schema_version"] != float64(1) {
		t.Fatalf("schema_version = %v", raw["schema_version"])
	}
	env := raw["envs"].(map[string]any)["vazia"].(map[string]any)
	for _, field := range []string{"description", "tags", "created_at", "updated_at", "vars"} {
		if _, ok := env[field]; !ok {
			t.Errorf("field %s missing", field)
		}
	}
	if _, ok := env["tags"].([]any); !ok {
		t.Errorf("tags = %v, want array", env["tags"])
	}
	if _, ok := env["vars"].([]any); !ok {
		t.Errorf("vars = %v, want array", env["vars"])
	}
}

func TestFailureBeforeRenameKeepsVault(t *testing.T) {
	f := newInitialized(t)
	mustCreate(t, f.vault, sampleEnv("a"))
	before := readVaultFile(t, f)
	f.vault.rename = func(string, string) error { return errors.New("disco cheio") }
	if _, err := f.vault.Create(context.Background(), sampleEnv("b")); err == nil {
		t.Fatal("Create succeeded despite rename failure")
	}
	if !bytes.Equal(before, readVaultFile(t, f)) {
		t.Fatal("vault.enc changed after failed write")
	}
	entries, err := os.ReadDir(f.paths.Dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}
	env, err := f.reopen().Get(context.Background(), "a")
	if err != nil || !env.SameContent(sampleEnv("a")) {
		t.Fatalf("previous vault unreadable: %v", err)
	}
}

func TestConcurrentWritesSerialize(t *testing.T) {
	f := newInitialized(t)
	const writers = 8
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			env := Env{Name: fmt.Sprintf("env-%d", i), Vars: []Var{{Key: "N", Value: fmt.Sprint(i)}}}
			_, err := f.reopen().Create(context.Background(), env)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	envs, err := f.vault.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(envs) != writers {
		t.Fatalf("len(envs) = %d, want %d: lost updates", len(envs), writers)
	}
}

func TestLockTimeout(t *testing.T) {
	f := newInitialized(t)
	holder := flock.New(f.paths.Lock)
	if err := holder.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	defer func() { _ = holder.Unlock() }()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if _, err := f.reopen().Create(ctx, sampleEnv("a")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if _, err := f.reopen().List(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("List err = %v, want DeadlineExceeded", err)
	}
}
