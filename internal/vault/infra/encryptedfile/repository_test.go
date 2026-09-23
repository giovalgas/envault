package encryptedfile

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

	"github.com/giovalgas/envault/internal/shared/config"
	"github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
)

type (
	Env = domain.Env
	Var = domain.Var
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
	vault *Repository
	paths config.Paths
	clock *clock
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	paths := config.PathsIn(filepath.Join(t.TempDir(), "envault"))
	c := &clock{now: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)}
	return &fixture{
		vault: New(paths, keystore.NewFile(paths.Key), WithClock(c.Now)),
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

func (f *fixture) reopen() *Repository {
	return New(f.paths, keystore.NewFile(f.paths.Key), WithClock(f.clock.Now))
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

func create(ctx context.Context, r *Repository, env Env) (Env, error) {
	var created Env
	err := r.Update(ctx, func(s *domain.Snapshot) error {
		var err error
		created, err = s.Create(env, r.clock.Now())
		return err
	})
	return created, err
}

func list(ctx context.Context, r *Repository) ([]Env, error) {
	snapshot, err := r.Load(ctx)
	if err != nil {
		return nil, err
	}
	return snapshot.Sorted(), nil
}

func get(ctx context.Context, r *Repository, name string) (Env, error) {
	snapshot, err := r.Load(ctx)
	if err != nil {
		return Env{}, err
	}
	return snapshot.Get(name)
}

func mustCreate(t *testing.T, r *Repository, env Env) Env {
	t.Helper()
	created, err := create(context.Background(), r, env)
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
	key, err := keystore.NewFile(f.paths.Key).Load(context.Background())
	if err != nil {
		t.Fatalf("Load key: %v", err)
	}
	sealed, err := Seal(key, []byte(plaintext))
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
	envs, err := list(context.Background(), f.vault)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(envs) != 0 {
		t.Fatalf("len(envs) = %d", len(envs))
	}
	if f.vault.Paths() != f.paths || f.vault.Location() != f.paths.Dir {
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
	if !errors.Is(err, domain.ErrDecrypt) || !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("err = %v, want domain.ErrDecrypt and ErrNoKey", err)
	}
	if _, err := os.Stat(f.paths.Key); err == nil {
		t.Fatal("Init recreated the key over an existing vault")
	}
}

func TestInitWithExistingKeyWithoutVault(t *testing.T) {
	f := newFixture(t)
	key, err := keystore.NewFile(f.paths.Key).Create(context.Background())
	if err != nil {
		t.Fatalf("Create key: %v", err)
	}
	if _, err := f.vault.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	loaded, err := keystore.NewFile(f.paths.Key).Load(context.Background())
	if err != nil || !bytes.Equal(key, loaded) {
		t.Fatalf("key changed: %v", err)
	}
}

func TestInitErrors(t *testing.T) {
	boom := errors.New("boom")
	paths := config.PathsIn(filepath.Join(t.TempDir(), "envault"))
	if _, err := New(paths, brokenKeys{loadErr: domain.ErrNoKey, createErr: boom}).Init(context.Background()); !errors.Is(err, boom) {
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
	if _, err := New(blocked, keystore.NewFile(blocked.Key)).Init(context.Background()); err == nil {
		t.Fatal("Init succeeded under a regular file")
	}
}

func TestInitOverWrongKeyFails(t *testing.T) {
	f := newInitialized(t)
	other, err := keystore.NewStatic(bytes.Repeat([]byte{5}, domain.KeySize))
	if err != nil {
		t.Fatalf("NewStatic: %v", err)
	}
	if _, err := New(f.paths, other).Init(context.Background()); !errors.Is(err, domain.ErrDecrypt) {
		t.Fatalf("err = %v, want domain.ErrDecrypt", err)
	}
}

func TestOperationsRequireInit(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	checks := map[string]error{}
	_, checks["list"] = list(ctx, f.vault)
	_, checks["get"] = get(ctx, f.vault, "a")
	_, checks["create"] = create(ctx, f.vault, sampleEnv("a"))
	checks["update"] = f.vault.Update(ctx, func(*domain.Snapshot) error { return nil })
	for op, err := range checks {
		if !errors.Is(err, domain.ErrNotInitialized) {
			t.Errorf("%s err = %v, want domain.ErrNotInitialized", op, err)
		}
	}
	if _, err := os.Stat(f.paths.Dir); err == nil {
		t.Fatal("operations without init created the vault dir")
	}
	exists, err := f.vault.Exists(context.Background())
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
	env, err := get(context.Background(), f.reopen(), "a")
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
		{"nome inválido", Env{Name: "Postgres Local"}, domain.ErrInvalidName, "Postgres Local"},
		{"chave duplicada", Env{Name: "a", Vars: []Var{{Key: "A", Value: "1"}, {Key: "A", Value: "2"}}}, domain.ErrInvalidKey, "A"},
		{"chave inválida", Env{Name: "a", Vars: []Var{{Key: "1KEY", Value: "x"}}}, domain.ErrInvalidKey, "1KEY"},
		{"tag com espaço", Env{Name: "a", Tags: []string{"Local DB"}}, domain.ErrInvalidTag, "Local DB"},
		{"descrição multilinha", Env{Name: "a", Description: "a\nb"}, domain.ErrInvalidDescription, "quebra"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := create(context.Background(), f.vault, tc.env)
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

func TestListSortedAndSnapshot(t *testing.T) {
	f := newInitialized(t)
	for _, name := range []string{"stripe-test", "aws-dev", "postgres-local"} {
		mustCreate(t, f.vault, sampleEnv(name))
	}
	envs, err := list(context.Background(), f.vault)
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
	if !snapshot.UpdatedAt.Equal(f.clock.Now()) {
		t.Fatalf("snapshot updated_at = %v", snapshot.UpdatedAt)
	}
}

func TestUpdatesPersistAcrossReopen(t *testing.T) {
	f := newInitialized(t)
	ctx := context.Background()
	original := mustCreate(t, f.vault, sampleEnv("a"))
	mustCreate(t, f.vault, sampleEnv("b"))
	f.clock.Advance(time.Hour)
	err := f.vault.Update(ctx, func(s *domain.Snapshot) error {
		if _, err := s.Rename("a", "c", f.clock.Now()); err != nil {
			return err
		}
		if _, err := s.Modify("c", f.clock.Now(), func(e *Env) error { e.Set("D", "4"); return nil }); err != nil {
			return err
		}
		return s.Delete("b")
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	envs, err := list(ctx, f.reopen())
	if err != nil || len(envs) != 1 {
		t.Fatalf("List = %+v, %v", envs, err)
	}
	got := envs[0]
	if got.Name != "c" || strings.Join(got.Keys(), ",") != "B,A,C,D" {
		t.Fatalf("env = %+v", got)
	}
	if !got.CreatedAt.Equal(original.CreatedAt) || !got.UpdatedAt.Equal(f.clock.Now()) {
		t.Fatalf("timestamps = %v, %v", got.CreatedAt, got.UpdatedAt)
	}
	if _, err := get(ctx, f.vault, "a"); !errors.Is(err, domain.ErrEnvNotFound) {
		t.Fatalf("old name still exists: %v", err)
	}
}

func TestUpdateValidatesAndPropagates(t *testing.T) {
	f := newInitialized(t)
	ctx := context.Background()
	boom := errors.New("boom")
	if err := f.vault.Update(ctx, func(*domain.Snapshot) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	err := f.vault.Update(ctx, func(s *domain.Snapshot) error {
		s.Envs["Bad Name"] = Env{}
		return nil
	})
	if !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("err = %v, want domain.ErrInvalidName", err)
	}
	err = f.vault.Update(ctx, func(s *domain.Snapshot) error {
		s.Envs["ok"] = Env{Name: "ignored", Vars: []Var{{Key: "K", Value: "v"}}}
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	env, err := get(ctx, f.vault, "ok")
	if err != nil || env.Name != "ok" {
		t.Fatalf("Get = %+v, %v", env, err)
	}
}

func TestWrongKeyFails(t *testing.T) {
	f := newInitialized(t)
	mustCreate(t, f.vault, sampleEnv("a"))
	other, err := keystore.NewStatic(bytes.Repeat([]byte{4}, domain.KeySize))
	if err != nil {
		t.Fatalf("NewStatic: %v", err)
	}
	if _, err := list(context.Background(), New(f.paths, other)); !errors.Is(err, domain.ErrDecrypt) {
		t.Fatalf("err = %v, want domain.ErrDecrypt", err)
	}
}

func TestStaticKeyOpensVault(t *testing.T) {
	paths := config.PathsIn(filepath.Join(t.TempDir(), "envault"))
	key := bytes.Repeat([]byte{8}, domain.KeySize)
	ks, err := keystore.WithOverride(keystore.EncodeKey(key), keystore.NewFile(paths.Key))
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
	if _, err := get(context.Background(), v, "a"); err != nil {
		t.Fatalf("Get: %v", err)
	}
}

func TestCorruptedVault(t *testing.T) {
	f := newInitialized(t)
	if err := os.WriteFile(f.paths.Vault, []byte("lixo"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := list(context.Background(), f.vault); !errors.Is(err, domain.ErrDecrypt) {
		t.Fatalf("err = %v, want domain.ErrDecrypt", err)
	}
}

func TestMissingKeyWithVault(t *testing.T) {
	f := newInitialized(t)
	if err := os.Remove(f.paths.Key); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_, err := list(context.Background(), f.vault)
	if !errors.Is(err, domain.ErrDecrypt) || !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("err = %v", err)
	}
}

func TestMalformedKeyIsNotDecrypt(t *testing.T) {
	f := newInitialized(t)
	if err := os.WriteFile(f.paths.Key, []byte("curta"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := list(context.Background(), f.vault)
	if !errors.Is(err, domain.ErrMalformedKey) || errors.Is(err, domain.ErrDecrypt) {
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
		{"versão futura", `{"schema_version":2,"envs":{}}`, domain.ErrUnsupportedSchema, "2"},
		{"sem versão", `{"envs":{}}`, domain.ErrDecrypt, "schema_version"},
		{"json inválido", `{`, domain.ErrDecrypt, "corrompido"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newInitialized(t)
			writeSealed(t, f, tc.plaintext)
			_, err := list(context.Background(), f.vault)
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
	envs, err := list(context.Background(), f.vault)
	if err != nil || len(envs) != 0 {
		t.Fatalf("List = %v, %v", envs, err)
	}
	mustCreate(t, f.vault, sampleEnv("a"))
}

func TestStoredJSONShape(t *testing.T) {
	f := newInitialized(t)
	mustCreate(t, f.vault, Env{Name: "vazia"})
	key, err := keystore.NewFile(f.paths.Key).Load(context.Background())
	if err != nil {
		t.Fatalf("Load key: %v", err)
	}
	plaintext, err := Open(key, readVaultFile(t, f))
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
	if _, err := create(context.Background(), f.vault, sampleEnv("b")); err == nil {
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
	env, err := get(context.Background(), f.reopen(), "a")
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
			_, err := create(context.Background(), f.reopen(), env)
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
	envs, err := list(context.Background(), f.vault)
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
	if _, err := create(ctx, f.reopen(), sampleEnv("a")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if _, err := list(ctx, f.reopen()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("List err = %v, want DeadlineExceeded", err)
	}
}
