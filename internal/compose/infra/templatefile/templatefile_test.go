package templatefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTemplateFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.example")
	if err := os.WriteFile(path, []byte("X=\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, found, err := New().ReadTemplate(path)
	if err != nil || !found || string(data) != "X=\n" {
		t.Fatalf("ReadTemplate = %q, %v, %v", data, found, err)
	}
}

func TestReadTemplateMissing(t *testing.T) {
	data, found, err := New().ReadTemplate(filepath.Join(t.TempDir(), ".env.example"))
	if err != nil || found || data != nil {
		t.Fatalf("ReadTemplate = %q, %v, %v", data, found, err)
	}
}

func TestReadTemplateDirectoryIsError(t *testing.T) {
	_, found, err := New().ReadTemplate(t.TempDir())
	if err == nil || found {
		t.Fatalf("ReadTemplate on directory = %v, %v", found, err)
	}
}
