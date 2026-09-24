package encryptedfile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofrs/flock"

	"github.com/giovalgas/envault/internal/shared/config"
	"github.com/giovalgas/envault/internal/vault/domain"
)

const lockRetryDelay = 20 * time.Millisecond

type Option func(*Repository)

func WithClock(now func() time.Time) Option {
	return func(r *Repository) { r.clock = now }
}

type Repository struct {
	paths  config.Paths
	keys   domain.KeyStore
	clock  domain.Clock
	rename func(oldpath, newpath string) error
}

var _ domain.EnvRepository = (*Repository)(nil)

func New(paths config.Paths, keys domain.KeyStore, opts ...Option) *Repository {
	r := &Repository{
		paths:  paths,
		keys:   keys,
		clock:  time.Now,
		rename: os.Rename,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Repository) Paths() config.Paths {
	return r.paths
}

func (r *Repository) Location() string {
	return r.paths.Dir
}

func (r *Repository) Exists(context.Context) (bool, error) {
	_, err := os.Stat(r.paths.Vault)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("verificar cofre: %w", err)
	}
	return true, nil
}

func (r *Repository) Init(ctx context.Context) (bool, error) {
	if err := os.MkdirAll(r.paths.Dir, config.DirPerm); err != nil {
		return false, fmt.Errorf("criar diretório do cofre: %w", err)
	}
	unlock, err := r.lock(ctx, false)
	if err != nil {
		return false, err
	}
	defer unlock()

	exists, err := r.Exists(ctx)
	if err != nil {
		return false, err
	}
	key, err := r.keys.Load(ctx)
	if errors.Is(err, domain.ErrNoKey) && !exists {
		key, err = r.keys.Create(ctx)
	}
	if err != nil {
		return false, keyError(err)
	}
	if exists {
		_, err := r.read(key)
		return false, err
	}
	if err := r.write(key, domain.NewSnapshot(r.clock.Now())); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) Load(ctx context.Context) (*domain.Snapshot, error) {
	if err := r.requireInitialized(ctx); err != nil {
		return nil, err
	}
	unlock, err := r.lock(ctx, true)
	if err != nil {
		return nil, err
	}
	defer unlock()
	key, err := r.key(ctx)
	if err != nil {
		return nil, err
	}
	return r.read(key)
}

func (r *Repository) Update(ctx context.Context, apply func(*domain.Snapshot) error) error {
	if err := r.requireInitialized(ctx); err != nil {
		return err
	}
	unlock, err := r.lock(ctx, false)
	if err != nil {
		return err
	}
	defer unlock()
	key, err := r.key(ctx)
	if err != nil {
		return err
	}
	snapshot, err := r.read(key)
	if err != nil {
		return err
	}
	if err := apply(snapshot); err != nil {
		return err
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	snapshot.UpdatedAt = r.clock.Now()
	return r.write(key, snapshot)
}

func (r *Repository) requireInitialized(ctx context.Context) error {
	exists, err := r.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: %s", domain.ErrNotInitialized, r.paths.Dir)
	}
	return nil
}

func (r *Repository) lock(ctx context.Context, shared bool) (func(), error) {
	fl := flock.New(r.paths.Lock, flock.SetPermissions(config.FilePerm))
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

func (r *Repository) key(ctx context.Context) ([]byte, error) {
	key, err := r.keys.Load(ctx)
	if err != nil {
		return nil, keyError(err)
	}
	return key, nil
}

func keyError(err error) error {
	if errors.Is(err, domain.ErrNoKey) {
		return fmt.Errorf("%w: %w", domain.ErrDecrypt, err)
	}
	return fmt.Errorf("obter chave: %w", err)
}

func (r *Repository) read(key []byte) (*domain.Snapshot, error) {
	sealed, err := os.ReadFile(r.paths.Vault)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", domain.ErrNotInitialized, r.paths.Dir)
	}
	if err != nil {
		return nil, fmt.Errorf("ler cofre: %w", err)
	}
	plaintext, err := Open(key, sealed)
	if err != nil {
		return nil, err
	}
	return decode(plaintext)
}

func (r *Repository) write(key []byte, snapshot *domain.Snapshot) error {
	plaintext, err := encode(snapshot)
	if err != nil {
		return fmt.Errorf("serializar cofre: %w", err)
	}
	sealed, err := Seal(key, plaintext)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(r.paths.Dir, ".vault-*.tmp")
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
	if err := r.rename(tmpPath, r.paths.Vault); err != nil {
		return fmt.Errorf("substituir cofre: %w", err)
	}
	committed = true
	syncDir(r.paths.Dir)
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
