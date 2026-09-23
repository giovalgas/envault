package keystore

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type fakeStore struct {
	key       []byte
	loadErr   error
	createErr error
	created   int
}

func (f *fakeStore) Load(context.Context) ([]byte, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	return f.key, nil
}

func (f *fakeStore) Create(context.Context) ([]byte, error) {
	f.created++
	return f.key, f.createErr
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestFileLoadWithoutKey(t *testing.T) {
	fs := NewFile(filepath.Join(t.TempDir(), "key"))
	if _, err := fs.Load(context.Background()); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("err = %v, want domain.ErrNoKey", err)
	}
}

func TestFileCreateAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "key")
	fs := NewFile(path)
	if fs.Path() != filepath.Clean(path) {
		t.Fatalf("Path = %q", fs.Path())
	}
	created, err := fs.Create(context.Background())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(created) != domain.KeySize {
		t.Fatalf("len = %d", len(created))
	}
	loaded, err := fs.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(created, loaded) {
		t.Fatal("loaded key differs from created key")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.TrimSpace(string(raw)) != EncodeKey(created) {
		t.Fatalf("file content is not base64 of the key")
	}
}

func TestFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissões Unix não se aplicam no Windows")
	}
	dir := filepath.Join(t.TempDir(), "home")
	path := filepath.Join(dir, "key")
	if _, err := NewFile(path).Create(context.Background()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key perm = %o, want 600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("dir perm = %o, want 700", dirInfo.Mode().Perm())
	}
}

func TestFileCreateTwiceKeepsKey(t *testing.T) {
	fs := NewFile(filepath.Join(t.TempDir(), "key"))
	first, err := fs.Create(context.Background())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := fs.Create(context.Background()); !errors.Is(err, domain.ErrKeyExists) {
		t.Fatalf("second Create err = %v, want domain.ErrKeyExists", err)
	}
	loaded, err := fs.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(first, loaded) {
		t.Fatal("second Create overwrote the key")
	}
}

func TestFileMalformed(t *testing.T) {
	cases := map[string]string{
		"not base64": "!!!",
		"short key":  EncodeKey(make([]byte, 16)),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "key")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			if _, err := NewFile(path).Load(context.Background()); !errors.Is(err, domain.ErrMalformedKey) {
				t.Fatalf("err = %v, want domain.ErrMalformedKey", err)
			}
		})
	}
}

func TestFileLoadReadError(t *testing.T) {
	dir := t.TempDir()
	if _, err := NewFile(dir).Load(context.Background()); err == nil || errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("err = %v, want read error", err)
	}
}

func TestFileCreateDirError(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := NewFile(filepath.Join(blocker, "key")).Create(context.Background()); err == nil {
		t.Fatal("Create succeeded under a regular file")
	}
}

func TestFileRemove(t *testing.T) {
	fs := NewFile(filepath.Join(t.TempDir(), "key"))
	if err := fs.Remove(context.Background()); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("Remove missing err = %v, want domain.ErrNoKey", err)
	}
	if _, err := fs.Create(context.Background()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := fs.Remove(context.Background()); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := fs.Load(context.Background()); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("Load after Remove err = %v, want domain.ErrNoKey", err)
	}
}

func TestFileRemoveError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "child"), nil, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := NewFile(dir).Remove(context.Background()); err == nil || errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("err = %v, want removal error", err)
	}
}

func TestFileHonorsCanceledContext(t *testing.T) {
	fs := NewFile(filepath.Join(t.TempDir(), "key"))
	ctx := canceledContext()
	if _, err := fs.Load(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load err = %v", err)
	}
	if _, err := fs.Create(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Create err = %v", err)
	}
	if err := fs.Remove(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Remove err = %v", err)
	}
}

func TestDecodeKey(t *testing.T) {
	key := bytes.Repeat([]byte{7}, domain.KeySize)
	decoded, err := DecodeKey("  " + EncodeKey(key) + "\n")
	if err != nil {
		t.Fatalf("DecodeKey: %v", err)
	}
	if !bytes.Equal(decoded, key) {
		t.Fatal("decoded key differs")
	}
	for _, bad := range []string{"", "@@", EncodeKey(make([]byte, 33))} {
		if _, err := DecodeKey(bad); !errors.Is(err, domain.ErrMalformedKey) {
			t.Fatalf("DecodeKey(%q) err = %v, want domain.ErrMalformedKey", bad, err)
		}
	}
}

func TestWithOverride(t *testing.T) {
	fallback := &fakeStore{}
	ks, err := WithOverride("  ", fallback)
	if err != nil {
		t.Fatalf("WithOverride empty: %v", err)
	}
	if ks != fallback {
		t.Fatal("empty override must return fallback")
	}

	key := bytes.Repeat([]byte{9}, domain.KeySize)
	ks, err = WithOverride(EncodeKey(key), fallback)
	if err != nil {
		t.Fatalf("WithOverride: %v", err)
	}
	loaded, err := ks.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(loaded, key) {
		t.Fatal("override key differs")
	}
	if _, err := ks.Create(context.Background()); !errors.Is(err, domain.ErrKeyExists) {
		t.Fatalf("Create err = %v, want domain.ErrKeyExists", err)
	}
	if _, err := ks.Load(canceledContext()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load canceled err = %v", err)
	}

	if _, err := WithOverride(EncodeKey(make([]byte, 8)), fallback); !errors.Is(err, domain.ErrMalformedKey) {
		t.Fatalf("short override err = %v, want domain.ErrMalformedKey", err)
	}
}

func TestOverrideBeatsFileKey(t *testing.T) {
	file := NewFile(filepath.Join(t.TempDir(), "key"))
	if _, err := file.Create(context.Background()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	override := bytes.Repeat([]byte{3}, domain.KeySize)
	ks, err := WithOverride(EncodeKey(override), file)
	if err != nil {
		t.Fatalf("WithOverride: %v", err)
	}
	loaded, err := ks.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(loaded, override) {
		t.Fatal("override did not win over file key")
	}
}

func TestNewStaticValidatesSize(t *testing.T) {
	if _, err := NewStatic(make([]byte, 5)); !errors.Is(err, domain.ErrMalformedKey) {
		t.Fatalf("err = %v, want domain.ErrMalformedKey", err)
	}
}

func TestChain(t *testing.T) {
	key := bytes.Repeat([]byte{1}, domain.KeySize)
	missing := &fakeStore{loadErr: domain.ErrNoKey}
	present := &fakeStore{key: key}
	loaded, err := Chain{missing, present}.Load(context.Background())
	if err != nil || !bytes.Equal(loaded, key) {
		t.Fatalf("Load = %v, %v", loaded, err)
	}
	if _, err := (Chain{missing, missing}).Load(context.Background()); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("all missing err = %v, want domain.ErrNoKey", err)
	}
	boom := errors.New("boom")
	if _, err := (Chain{&fakeStore{loadErr: boom}, present}).Load(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	first := &fakeStore{key: key}
	second := &fakeStore{key: key}
	if _, err := (Chain{first, second}).Create(context.Background()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if first.created != 1 || second.created != 0 {
		t.Fatalf("created = %d, %d", first.created, second.created)
	}
	if _, err := (Chain{}).Create(context.Background()); err == nil {
		t.Fatal("empty chain Create succeeded")
	}
}
