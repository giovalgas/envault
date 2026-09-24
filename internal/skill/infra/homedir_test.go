package infra

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSkillHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	h := NewHomeDir()
	dir, err := h.Dir(context.Background())
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if dir != filepath.Join(home, ".claude", "skills") {
		t.Fatalf("dir = %q", dir)
	}
}

func TestSkillHomeDirCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := NewHomeDir()
	if _, err := h.Dir(ctx); err == nil {
		t.Fatal("Dir com contexto cancelado sem erro")
	}
}
