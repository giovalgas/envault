package envfile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/compose/domain"
	"github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/shared/dotenv"
)

func loadTestVars(pairs ...string) []domain.Var {
	out := []domain.Var{}
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, domain.Var{Key: pairs[i], Value: pairs[i+1]})
	}
	return out
}

func loadTestWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func loadTestRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

func loadTestEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func TestLoadEnvFileExists(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".env")
	if exists, err := New().Exists(target); err != nil || exists {
		t.Fatalf("Exists(missing) = %v, %v", exists, err)
	}
	loadTestWrite(t, target, "A=1\n")
	if exists, err := New().Exists(target); err != nil || !exists {
		t.Fatalf("Exists(file) = %v, %v", exists, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	dangling := filepath.Join(dir, "link.env")
	if err := os.Symlink("nao-existe", dangling); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if exists, err := New().Exists(dangling); err != nil || !exists {
		t.Fatalf("Exists(dangling) = %v, %v", exists, err)
	}
}

func TestLoadEnvFileExistsError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "arquivo")
	loadTestWrite(t, file, "")
	_, err := New().Exists(filepath.Join(file, ".env"))
	if runtime.GOOS == "windows" {
		return
	}
	if err == nil || !strings.Contains(err.Error(), "inspecionar destino") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadEnvFileWrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".env")
	loadTestWrite(t, target, "# descrição\nOLD=1\n")
	if err := New().Write(target, loadTestVars("A", "1", "B", "a b")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := loadTestRead(t, target); got != "A=1\nB=\"a b\"\n" {
		t.Fatalf(".env = %q", got)
	}
	if entries := loadTestEntries(t, dir); !slices.Equal(entries, []string{".env"}) {
		t.Fatalf("entries = %v", entries)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("perm = %o", perm)
		}
	}
}

func TestLoadEnvFileWriteErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".env"), 0o700); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	err := New().Write(filepath.Join(dir, ".env"), loadTestVars("A", "1"))
	if !errors.Is(err, usecase.ErrTargetIsDirectory) {
		t.Fatalf("directory err = %v", err)
	}
	err = New().Write(filepath.Join(dir, "nao", "existe", ".env"), loadTestVars("A", "1"))
	if err == nil || !strings.Contains(err.Error(), "gravar") {
		t.Fatalf("missing dir err = %v", err)
	}
}

func TestLoadEnvFileFollowsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink exige privilégio no Windows")
	}
	dir := t.TempDir()
	realPath := filepath.Join(dir, "real.env")
	loadTestWrite(t, realPath, "OLD=1\n")
	link := filepath.Join(dir, ".env")
	if err := os.Symlink("real.env", link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if err := New().Write(link, loadTestVars("A", "1")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v %v", info, err)
	}
	if got := loadTestRead(t, realPath); got != "A=1\n" {
		t.Fatalf("real.env = %q", got)
	}
	dangling := filepath.Join(dir, "dangling.env")
	if err := os.Symlink(filepath.Join("nao", "existe"), dangling); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if err := New().Write(dangling, loadTestVars("A", "1")); err == nil || !strings.Contains(err.Error(), "resolver link") {
		t.Fatalf("dangling err = %v", err)
	}
}

func TestLoadEnvFileMerge(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".env")
	loadTestWrite(t, target, "A=old\nLOCAL=1\n")
	written, err := New().Merge(target, loadTestVars("A", "new", "B", "2"))
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if written != 3 {
		t.Fatalf("written = %d", written)
	}
	if got := loadTestRead(t, target); got != "A=new\nLOCAL=1\nB=2\n" {
		t.Fatalf(".env = %q", got)
	}
}

func TestLoadEnvFileMergeKeepsMetadata(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".env")
	original := dotenv.Format(dotenv.Document{Description: "local", Tags: []string{"dev"}, Vars: []dotenv.Var{{Key: "A", Value: "old"}}})
	loadTestWrite(t, target, string(original))
	if _, err := New().Merge(target, loadTestVars("A", "new")); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	doc, err := dotenv.Parse([]byte(loadTestRead(t, target)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Description != "local" || !slices.Equal(doc.Tags, []string{"dev"}) || !slices.Equal(doc.Vars, []dotenv.Var{{Key: "A", Value: "new"}}) {
		t.Fatalf("doc = %+v", doc)
	}
}

func TestLoadEnvFileMergeErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := New().Merge(filepath.Join(dir, "nao-existe"), nil); err == nil || !strings.Contains(err.Error(), "ler destino") {
		t.Fatalf("read err = %v", err)
	}
	invalid := filepath.Join(dir, "invalid.env")
	loadTestWrite(t, invalid, "NOT VALID\n")
	_, err := New().Merge(invalid, loadTestVars("A", "1"))
	if !errors.Is(err, dotenv.ErrSyntax) || !strings.Contains(err.Error(), "mesclar com") {
		t.Fatalf("parse err = %v", err)
	}
	if got := loadTestRead(t, invalid); got != "NOT VALID\n" {
		t.Fatalf("invalid.env = %q", got)
	}
}
