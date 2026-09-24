package sourcefile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestReadEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vars.env")
	if err := os.WriteFile(path, []byte("A=1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := New().ReadEnvFile(path)
	if err != nil || string(data) != "A=1\n" {
		t.Fatalf("ReadEnvFile = %q, %v", data, err)
	}
}

func TestReadEnvFileMissing(t *testing.T) {
	_, err := New().ReadEnvFile(filepath.Join(t.TempDir(), "nada.env"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want ErrNotExist", err)
	}
}
