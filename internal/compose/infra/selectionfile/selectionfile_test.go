package selectionfile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovalgas/envault/internal/compose/domain"
	"github.com/giovalgas/envault/internal/shared/config"
)

var savedAt = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	return New(func() (string, error) { return dir, nil }), filepath.Join(dir, config.SelectionFileName)
}

func TestLoadWithoutFileIsEmpty(t *testing.T) {
	store, path := newStore(t)
	sel, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sel.Envs == nil || len(sel.Envs) != 0 || sel.Saved() {
		t.Fatalf("seleção = %+v", sel)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Load não deveria criar o arquivo: %v", err)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()
	if err := store.Save(ctx, domain.Selection{Envs: []string{"b", "a"}, UpdatedAt: savedAt}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	sel, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !slices.Equal(sel.Envs, []string{"b", "a"}) || !sel.UpdatedAt.Equal(savedAt) {
		t.Fatalf("seleção = %+v", sel)
	}
}

func TestSavedFileHasOnlyNames(t *testing.T) {
	store, path := newStore(t)
	if err := store.Save(context.Background(), domain.Selection{Envs: []string{"a", "b"}, UpdatedAt: savedAt}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, data)
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"envs", "schema_version", "updated_at"}) {
		t.Fatalf("campos = %v", keys)
	}
	if string(raw["envs"]) != "[\n    \"a\",\n    \"b\"\n  ]" || string(raw["schema_version"]) != "1" {
		t.Fatalf("conteúdo = %s", data)
	}
}

func TestSaveUsesPrivatePermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissões POSIX não se aplicam no windows")
	}
	store, path := newStore(t)
	for range 2 {
		if err := store.Save(context.Background(), domain.Selection{Envs: []string{"a"}, UpdatedAt: savedAt}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != config.FilePerm {
			t.Fatalf("permissão = %o, esperado %o", perm, config.FilePerm)
		}
	}
}

func TestSaveEmptySelectionWritesEmptyList(t *testing.T) {
	store, path := newStore(t)
	if err := store.Save(context.Background(), domain.Selection{UpdatedAt: savedAt}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), `"envs": []`) {
		t.Fatalf("conteúdo = %s", data)
	}
}

func TestSaveLeavesNoTemporaryFile(t *testing.T) {
	store, path := newStore(t)
	if err := store.Save(context.Background(), domain.Selection{Envs: []string{"a"}, UpdatedAt: savedAt}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != config.SelectionFileName {
		t.Fatalf("arquivos = %v", entries)
	}
}

func TestSaveIntoMissingDirFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nao-existe")
	store := New(func() (string, error) { return missing, nil })
	if err := store.Save(context.Background(), domain.Selection{}); err == nil {
		t.Fatal("Save deveria falhar sem o diretório do cofre")
	}
}

func TestLoadRejectsBadContent(t *testing.T) {
	cases := map[string]string{
		"json":    "{",
		"schema":  `{"schema_version":2,"envs":[]}`,
		"repeats": `{"schema_version":1,"envs":["a","a"]}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			store, path := newStore(t)
			if err := os.WriteFile(path, []byte(content), config.FilePerm); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			if _, err := store.Load(context.Background()); err == nil {
				t.Fatal("Load deveria falhar")
			}
		})
	}
	store, path := newStore(t)
	if err := os.WriteFile(path, []byte(`{"schema_version":2}`), config.FilePerm); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := store.Load(context.Background()); !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("err = %v", err)
	}
}

func TestLocateErrorPropagates(t *testing.T) {
	boom := errors.New("boom")
	store := New(func() (string, error) { return "", boom })
	if _, err := store.Load(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("Load err = %v", err)
	}
	if err := store.Save(context.Background(), domain.Selection{}); !errors.Is(err, boom) {
		t.Fatalf("Save err = %v", err)
	}
}
