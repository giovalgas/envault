package infra

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSkillFileWriterCreatesDirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")
	target := filepath.Join(dir, "envault", "SKILL.md")
	writer := NewFileWriter()
	if err := writer.Write(context.Background(), target, []byte("conteúdo")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ler instalado: %v", err)
	}
	if string(got) != "conteúdo" {
		t.Fatalf("conteúdo = %q", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != filePerm {
			t.Fatalf("perm = %o", info.Mode().Perm())
		}
	}
}

func TestSkillFileWriterOverwrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "envault", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("conteúdo antigo"+strings.Repeat("x", 10000)), 0o644); err != nil {
		t.Fatal(err)
	}
	writer := NewFileWriter()
	if err := writer.Write(context.Background(), target, []byte("novo")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "novo" {
		t.Fatal("reinstalação não sobrescreveu")
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("sobrou arquivo temporário: %v", entries)
	}
}

func TestSkillFileWriterCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	target := filepath.Join(t.TempDir(), "envault", "SKILL.md")
	writer := NewFileWriter()
	if err := writer.Write(ctx, target, []byte("x")); err == nil {
		t.Fatal("Write com contexto cancelado sem erro")
	}
}
