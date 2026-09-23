package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/gofrs/flock"

	"github.com/giovalgas/envault/internal/config"
	"github.com/giovalgas/envault/internal/crypto"
	"github.com/giovalgas/envault/internal/store"
)

const (
	SchemaVersion  = 1
	lockRetryDelay = 20 * time.Millisecond
)

type Snapshot struct {
	SchemaVersion int            `json:"schema_version"`
	UpdatedAt     time.Time      `json:"updated_at"`
	Envs          map[string]Env `json:"envs"`
}

func (s *Snapshot) Names() []string {
	names := make([]string, 0, len(s.Envs))
	for name := range s.Envs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *Snapshot) Sorted() []Env {
	envs := make([]Env, 0, len(s.Envs))
	for _, name := range s.Names() {
		envs = append(envs, s.Envs[name].Clone())
	}
	return envs
}

type Option func(*Vault)

func WithClock(now func() time.Time) Option {
	return func(v *Vault) { v.now = now }
}

type Vault struct {
	paths  config.Paths
	keys   store.KeyStore
	now    func() time.Time
	rename func(oldpath, newpath string) error
}

func New(paths config.Paths, keys store.KeyStore, opts ...Option) *Vault {
	v := &Vault{
		paths:  paths,
		keys:   keys,
		now:    time.Now,
		rename: os.Rename,
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

func (v *Vault) Paths() config.Paths {
	return v.paths
}

func (v *Vault) Exists() (bool, error) {
	_, err := os.Stat(v.paths.Vault)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("verificar cofre: %w", err)
	}
	return true, nil
}

func (v *Vault) Init(ctx context.Context) (bool, error) {
	if err := os.MkdirAll(v.paths.Dir, config.DirPerm); err != nil {
		return false, fmt.Errorf("criar diretório do cofre: %w", err)
	}
	unlock, err := v.lock(ctx, false)
	if err != nil {
		return false, err
	}
	defer unlock()

	exists, err := v.Exists()
	if err != nil {
		return false, err
	}
	key, err := v.keys.Load(ctx)
	if errors.Is(err, store.ErrNoKey) && !exists {
		key, err = v.keys.Create(ctx)
	}
	if err != nil {
		return false, v.keyError(err)
	}
	if exists {
		_, err := v.read(key)
		return false, err
	}
	snapshot := &Snapshot{SchemaVersion: SchemaVersion, UpdatedAt: v.timestamp(), Envs: map[string]Env{}}
	if err := v.write(key, snapshot); err != nil {
		return false, err
	}
	return true, nil
}

func (v *Vault) Load(ctx context.Context) (*Snapshot, error) {
	if err := v.requireInitialized(); err != nil {
		return nil, err
	}
	unlock, err := v.lock(ctx, true)
	if err != nil {
		return nil, err
	}
	defer unlock()
	key, err := v.key(ctx)
	if err != nil {
		return nil, err
	}
	return v.read(key)
}

func (v *Vault) Update(ctx context.Context, apply func(*Snapshot) error) error {
	if err := v.requireInitialized(); err != nil {
		return err
	}
	unlock, err := v.lock(ctx, false)
	if err != nil {
		return err
	}
	defer unlock()
	key, err := v.key(ctx)
	if err != nil {
		return err
	}
	snapshot, err := v.read(key)
	if err != nil {
		return err
	}
	if err := apply(snapshot); err != nil {
		return err
	}
	for name, env := range snapshot.Envs {
		env.Name = name
		if err := env.Validate(); err != nil {
			return err
		}
		snapshot.Envs[name] = env
	}
	snapshot.SchemaVersion = SchemaVersion
	snapshot.UpdatedAt = v.timestamp()
	return v.write(key, snapshot)
}

func (v *Vault) requireInitialized() error {
	exists, err := v.Exists()
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: %s", ErrNotInitialized, v.paths.Dir)
	}
	return nil
}

func (v *Vault) lock(ctx context.Context, shared bool) (func(), error) {
	fl := flock.New(v.paths.Lock, flock.SetPermissions(config.FilePerm))
	try := fl.TryLockContext
	if shared {
		try = fl.TryRLockContext
	}
	locked, err := try(ctx, lockRetryDelay)
	if err != nil {
		return nil, fmt.Errorf("adquirir lock do cofre: %w", err)
	}
	if !locked {
		return nil, errors.New("adquirir lock do cofre: não obtido")
	}
	return func() { _ = fl.Unlock() }, nil
}

func (v *Vault) key(ctx context.Context) ([]byte, error) {
	key, err := v.keys.Load(ctx)
	if err != nil {
		return nil, v.keyError(err)
	}
	return key, nil
}

func (v *Vault) keyError(err error) error {
	if errors.Is(err, store.ErrNoKey) {
		return fmt.Errorf("%w: %w", ErrDecrypt, err)
	}
	return fmt.Errorf("obter chave: %w", err)
}

func (v *Vault) read(key []byte) (*Snapshot, error) {
	sealed, err := os.ReadFile(v.paths.Vault)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotInitialized, v.paths.Dir)
	}
	if err != nil {
		return nil, fmt.Errorf("ler cofre: %w", err)
	}
	plaintext, err := crypto.Open(key, sealed)
	if err != nil {
		return nil, err
	}
	return decode(plaintext)
}

func decode(plaintext []byte) (*Snapshot, error) {
	var snapshot Snapshot
	if err := json.Unmarshal(plaintext, &snapshot); err != nil {
		return nil, fmt.Errorf("%w: conteúdo corrompido", ErrDecrypt)
	}
	if snapshot.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("%w: encontrada %d, suportada até %d", ErrUnsupportedSchema, snapshot.SchemaVersion, SchemaVersion)
	}
	if snapshot.SchemaVersion < 1 {
		return nil, fmt.Errorf("%w: schema_version ausente", ErrDecrypt)
	}
	if snapshot.Envs == nil {
		snapshot.Envs = map[string]Env{}
	}
	for name, env := range snapshot.Envs {
		env.Name = name
		snapshot.Envs[name] = env
	}
	return &snapshot, nil
}

func encode(snapshot *Snapshot) ([]byte, error) {
	out := Snapshot{
		SchemaVersion: snapshot.SchemaVersion,
		UpdatedAt:     snapshot.UpdatedAt,
		Envs:          make(map[string]Env, len(snapshot.Envs)),
	}
	for name, env := range snapshot.Envs {
		out.Envs[name] = env.normalized()
	}
	return json.Marshal(out)
}

func (v *Vault) write(key []byte, snapshot *Snapshot) error {
	plaintext, err := encode(snapshot)
	if err != nil {
		return fmt.Errorf("serializar cofre: %w", err)
	}
	sealed, err := crypto.Seal(key, plaintext)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(v.paths.Dir, ".vault-*.tmp")
	if err != nil {
		return fmt.Errorf("criar temporário do cofre: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(config.FilePerm); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("ajustar permissão do temporário: %w", err)
	}
	if _, err := tmp.Write(sealed); err != nil {
		return fmt.Errorf("gravar temporário do cofre: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sincronizar temporário do cofre: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fechar temporário do cofre: %w", err)
	}
	if err := v.rename(tmpPath, v.paths.Vault); err != nil {
		return fmt.Errorf("substituir cofre: %w", err)
	}
	committed = true
	syncDir(v.paths.Dir)
	return nil
}

func syncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	d, err := os.Open(filepath.Clean(dir))
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

func (v *Vault) timestamp() time.Time {
	return v.now().UTC().Truncate(time.Second)
}
