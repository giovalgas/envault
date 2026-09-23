package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/giovalgas/envault/internal/skill/domain"
)

type fakeWriter struct {
	path    string
	content []byte
	err     error
	calls   int
}

func (w *fakeWriter) Write(_ context.Context, path string, content []byte) error {
	w.calls++
	w.path = path
	w.content = content
	return w.err
}

type fakeHomeDir struct {
	dir string
	err error
}

func (h *fakeHomeDir) Dir(context.Context) (string, error) {
	return h.dir, h.err
}

func TestInstallSkillWithExplicitDir(t *testing.T) {
	writer := &fakeWriter{}
	home := &fakeHomeDir{err: errors.New("não deve ser chamado")}
	uc := NewInstallSkill(writer, home)
	path, err := uc.Execute(context.Background(), "/skills")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := filepath.Join("/skills", domain.Name, domain.FileName)
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if writer.calls != 1 || writer.path != want {
		t.Fatalf("writer = %+v", writer)
	}
	if string(writer.content) != domain.Content() {
		t.Fatal("conteúdo gravado difere de domain.Content()")
	}
}

func TestInstallSkillResolvesDefaultDir(t *testing.T) {
	writer := &fakeWriter{}
	home := &fakeHomeDir{dir: "/home/user/.claude/skills"}
	uc := NewInstallSkill(writer, home)
	path, err := uc.Execute(context.Background(), "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := filepath.Join("/home/user/.claude/skills", domain.Name, domain.FileName)
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestInstallSkillHomeDirError(t *testing.T) {
	writer := &fakeWriter{}
	home := &fakeHomeDir{err: domain.ErrNoHomeDir}
	uc := NewInstallSkill(writer, home)
	if _, err := uc.Execute(context.Background(), ""); !errors.Is(err, domain.ErrNoHomeDir) {
		t.Fatalf("err = %v, want %v", err, domain.ErrNoHomeDir)
	}
	if writer.calls != 0 {
		t.Fatalf("writer chamado sem diretório resolvido: %+v", writer)
	}
}

func TestInstallSkillWriterError(t *testing.T) {
	writer := &fakeWriter{err: errors.New("disco cheio")}
	home := &fakeHomeDir{dir: "/home/user/.claude/skills"}
	uc := NewInstallSkill(writer, home)
	if _, err := uc.Execute(context.Background(), "/skills"); err == nil {
		t.Fatal("Execute sem erro")
	}
}
