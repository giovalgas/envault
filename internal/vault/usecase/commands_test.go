package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giovalgas/envault/internal/vault/domain"
)

var later = domain.Clock(func() time.Time { return baseTime.Add(time.Hour) })

func TestSetValues(t *testing.T) {
	repo := newRepo(sampleEnv())
	uc := NewSetValues(repo, later)
	env, err := uc.Execute(context.Background(), "a", []VarView{{Key: "POOL", Value: "20"}, {Key: "NEW", Value: "x"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Join(env.Keys(), ",") != "DATABASE_URL,POOL,NEW" || !env.UpdatedAt.Equal(later.Now()) {
		t.Fatalf("env = %+v", env)
	}
	if value, _ := repo.env(t, "a").Lookup("POOL"); value != "20" {
		t.Fatalf("POOL = %q", value)
	}
	if _, err := uc.Execute(context.Background(), "a", []VarView{{Key: "OK", Value: "1"}, {Key: "1BAD", Value: "x"}}); !errors.Is(err, domain.ErrInvalidKey) {
		t.Fatalf("invalid err = %v", err)
	}
	if _, ok := repo.env(t, "a").Lookup("OK"); ok {
		t.Fatal("invalid set was partially applied")
	}
	if _, err := uc.Execute(context.Background(), "b", nil); !errors.Is(err, domain.ErrEnvNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}

func TestUnsetKeys(t *testing.T) {
	repo := newRepo(sampleEnv())
	uc := NewUnsetKeys(repo, later)
	removed, err := uc.Execute(context.Background(), "a", []string{"POOL", "NOPE"})
	if err != nil || strings.Join(removed, ",") != "POOL" {
		t.Fatalf("removed = %v, %v", removed, err)
	}
	if keys := repo.env(t, "a").Keys(); strings.Join(keys, ",") != "DATABASE_URL" {
		t.Fatalf("keys = %v", keys)
	}
	removed, err = uc.Execute(context.Background(), "a", []string{"NOPE"})
	if err != nil || len(removed) != 0 {
		t.Fatalf("removed = %v, %v", removed, err)
	}
	if _, err := uc.Execute(context.Background(), "b", []string{"X"}); !errors.Is(err, domain.ErrEnvNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}

func TestRenameEnv(t *testing.T) {
	repo := newRepo(sampleEnv())
	uc := NewRenameEnv(repo, later)
	env, err := uc.Execute(context.Background(), "a", "b")
	if err != nil || env.Name != "b" || !env.UpdatedAt.Equal(later.Now()) {
		t.Fatalf("env = %+v, %v", env, err)
	}
	if repo.snapshot.Has("a") || !repo.snapshot.Has("b") {
		t.Fatal("rename not applied")
	}
	uninitialized := &memRepo{}
	if _, err := NewRenameEnv(uninitialized, later).Execute(context.Background(), "a", "Bad Name"); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("name must be validated before the vault: %v", err)
	}
	if _, err := uc.Execute(context.Background(), "a", "c"); !errors.Is(err, domain.ErrEnvNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}

func TestCopyEnv(t *testing.T) {
	repo := newRepo(sampleEnv())
	uc := NewCopyEnv(repo, later)
	env, err := uc.Execute(context.Background(), "a", "b")
	if err != nil || env.Name != "b" || !env.CreatedAt.Equal(later.Now()) || !envFromView(env).SameContent(sampleEnv()) {
		t.Fatalf("env = %+v, %v", env, err)
	}
	if !repo.snapshot.Has("a") || !repo.snapshot.Has("b") {
		t.Fatal("copy not applied")
	}
	if _, err := NewCopyEnv(&memRepo{}, later).Execute(context.Background(), "a", "Bad Name"); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("name must be validated before the vault: %v", err)
	}
	if _, err := uc.Execute(context.Background(), "a", "b"); !errors.Is(err, domain.ErrEnvExists) {
		t.Fatalf("exists err = %v", err)
	}
}

func TestDeleteEnv(t *testing.T) {
	repo := newRepo(sampleEnv())
	uc := NewDeleteEnv(repo)
	if err := uc.Execute(context.Background(), "a"); err != nil || repo.snapshot.Has("a") {
		t.Fatalf("Execute = %v", err)
	}
	if err := uc.Execute(context.Background(), "a"); !errors.Is(err, domain.ErrEnvNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}

func TestCreateEnvWithEditor(t *testing.T) {
	repo := newRepo()
	editor := editTo(domain.Env{Description: "do editor", Tags: []string{"db"}, Vars: []domain.Var{{Key: "A", Value: "1"}}})
	uc := NewCreateEnv(repo, editor, nil, later)
	result, err := uc.Execute(context.Background(), CreateEnvInput{Name: "nova", Tags: []string{"db"}})
	if err != nil || !result.Created {
		t.Fatalf("result = %+v, %v", result, err)
	}
	if !editor.isNew || editor.seen.Name != "nova" || !editor.seen.HasTag("db") {
		t.Fatalf("editor saw %+v new=%v", editor.seen, editor.isNew)
	}
	stored := repo.env(t, "nova")
	if stored.Description != "do editor" || !stored.HasTag("db") || !stored.CreatedAt.Equal(later.Now()) {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestCreateEnvEditorUnchangedOrFails(t *testing.T) {
	repo := newRepo()
	unchanged := NewCreateEnv(repo, editTo(domain.Env{}), nil, later)
	result, err := unchanged.Execute(context.Background(), CreateEnvInput{Name: "nova"})
	if err != nil || result.Created || repo.updates != 0 {
		t.Fatalf("result = %+v, %v, updates %d", result, err, repo.updates)
	}
	failing := &fakeEditor{result: func(domain.Env) (domain.EditResult, error) {
		return domain.EditResult{}, domain.ErrEditCanceled
	}}
	if _, err := NewCreateEnv(repo, failing, nil, later).Execute(context.Background(), CreateEnvInput{Name: "nova"}); !errors.Is(err, domain.ErrEditCanceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateEnvChecksBeforeEditing(t *testing.T) {
	editor := editTo(domain.Env{Vars: []domain.Var{{Key: "A", Value: "1"}}})
	uc := NewCreateEnv(newRepo(sampleEnv()), editor, nil, later)
	if _, err := uc.Execute(context.Background(), CreateEnvInput{Name: "Bad Name"}); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("invalid err = %v", err)
	}
	if _, err := uc.Execute(context.Background(), CreateEnvInput{Name: "a"}); !errors.Is(err, domain.ErrEnvExists) {
		t.Fatalf("exists err = %v", err)
	}
	if _, err := NewCreateEnv(&memRepo{}, editor, nil, later).Execute(context.Background(), CreateEnvInput{Name: "b"}); !errors.Is(err, domain.ErrNotInitialized) {
		t.Fatalf("uninitialized err = %v", err)
	}
	if editor.calls != 0 {
		t.Fatalf("editor opened %d times", editor.calls)
	}
}

func TestCreateEnvFromSource(t *testing.T) {
	repo := newRepo()
	from := files("vars.env", "# @description: do arquivo\n# @tags: file\nA=1\n")
	uc := NewCreateEnv(repo, &fakeEditor{}, from, later)
	in := CreateEnvInput{Name: "a", Description: "da flag", Tags: []string{"flag"}, FromFile: "vars.env"}
	if _, err := uc.Execute(context.Background(), in); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if env := repo.env(t, "a"); env.Description != "do arquivo" || !env.HasTag("file") {
		t.Fatalf("without overrides = %+v", env)
	}
	in.Name = "b"
	in.OverrideDescription = true
	in.OverrideTags = true
	if _, err := uc.Execute(context.Background(), in); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if env := repo.env(t, "b"); env.Description != "da flag" || !env.HasTag("flag") || env.HasTag("file") {
		t.Fatalf("with overrides = %+v", env)
	}
	broken := NewCreateEnv(repo, &fakeEditor{}, files("ruim.env", "A=1\nsem igual\n"), later)
	in.Name = "c"
	in.FromFile = "ruim.env"
	if _, err := broken.Execute(context.Background(), in); err == nil || !strings.HasPrefix(err.Error(), "ruim.env: linha 2") {
		t.Fatalf("parse err = %v", err)
	}
	boom := errors.New("boom")
	failing := NewCreateEnv(repo, &fakeEditor{}, &fakeFiles{err: boom}, later)
	in.FromFile = "vars.env"
	if _, err := failing.Execute(context.Background(), in); !errors.Is(err, boom) || err.Error() != "ler vars.env: boom" {
		t.Fatalf("read err = %v", err)
	}
}

func TestImportEnv(t *testing.T) {
	repo := newRepo(domain.Env{Name: "a", Vars: []domain.Var{{Key: "OLD", Value: "1"}}})
	from := &fakeFiles{content: map[string]string{"b.env": "# @description: arquivo\nX=1\n", "a.env": "NEW=2\n"}}
	uc := NewImportEnv(repo, from, later)
	created, err := uc.Execute(context.Background(), ImportEnvInput{Name: "b", Path: "b.env"})
	if err != nil || created.Replaced || created.Env.Description != "arquivo" {
		t.Fatalf("created = %+v, %v", created, err)
	}
	description := "da flag"
	replaced, err := uc.Execute(context.Background(), ImportEnvInput{Name: "a", Path: "a.env", Description: &description})
	if err != nil || !replaced.Replaced {
		t.Fatalf("replaced = %+v, %v", replaced, err)
	}
	env := repo.env(t, "a")
	if strings.Join(env.Keys(), ",") != "NEW" || env.Description != "da flag" || !env.CreatedAt.Equal(baseTime) || !env.UpdatedAt.Equal(later.Now()) {
		t.Fatalf("env = %+v", env)
	}
}

func TestImportEnvResolvesPathInDir(t *testing.T) {
	dir := t.TempDir()
	inDir := filepath.Join(dir, "b.env")
	from := files(inDir, "X=1\n")
	if _, err := NewImportEnv(newRepo(), from, later).Execute(context.Background(), ImportEnvInput{Name: "b", Path: "b.env", Dir: dir}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	absolute := filepath.Join(t.TempDir(), "c.env")
	if _, err := NewImportEnv(newRepo(), from, later).Execute(context.Background(), ImportEnvInput{Name: "c", Path: absolute, Dir: dir}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Join(from.paths, ",") != inDir+","+absolute {
		t.Fatalf("paths = %v", from.paths)
	}
}

func TestImportEnvOrderOfErrors(t *testing.T) {
	counting := files("x.env", "A=1\n")
	if _, err := NewImportEnv(newRepo(), counting, later).Execute(context.Background(), ImportEnvInput{Name: "Bad Name", Path: "x.env"}); !errors.Is(err, domain.ErrInvalidName) || counting.reads != 0 {
		t.Fatalf("invalid name err = %v, reads %d", err, counting.reads)
	}
	if _, err := NewImportEnv(&memRepo{}, files("x.env", "lixo\n"), later).Execute(context.Background(), ImportEnvInput{Name: "a", Path: "x.env"}); err == nil || !strings.HasPrefix(err.Error(), "x.env: linha 1") {
		t.Fatalf("parse must precede the vault: %v", err)
	}
	if _, err := NewImportEnv(&memRepo{}, counting, later).Execute(context.Background(), ImportEnvInput{Name: "a", Path: "x.env"}); !errors.Is(err, domain.ErrNotInitialized) {
		t.Fatalf("uninitialized err = %v", err)
	}
	multiline := "a\nb"
	if _, err := NewImportEnv(newRepo(), counting, later).Execute(context.Background(), ImportEnvInput{Name: "a", Path: "x.env", Description: &multiline}); !errors.Is(err, domain.ErrInvalidDescription) {
		t.Fatalf("description err = %v", err)
	}
	boom := errors.New("boom")
	if _, err := NewImportEnv(newRepo(), &fakeFiles{err: boom}, later).Execute(context.Background(), ImportEnvInput{Name: "a", Path: "x.env"}); !errors.Is(err, boom) {
		t.Fatalf("read err = %v", err)
	}
}

func TestEditEnv(t *testing.T) {
	repo := newRepo(sampleEnv())
	editor := editTo(domain.Env{Description: "nova", Vars: []domain.Var{{Key: "POOL", Value: "20"}}})
	result, err := NewEditEnv(repo, editor, later).Execute(context.Background(), "a")
	if err != nil || !result.Changed || editor.isNew {
		t.Fatalf("result = %+v, %v", result, err)
	}
	if lines := strings.Join(result.Diff.Lines(), "|"); lines != "- DATABASE_URL|~ POOL|~ descrição ou tags" {
		t.Fatalf("diff = %q", lines)
	}
	env := repo.env(t, "a")
	if env.Description != "nova" || strings.Join(env.Keys(), ",") != "POOL" || !env.UpdatedAt.Equal(later.Now()) {
		t.Fatalf("env = %+v", env)
	}
}

func TestEditEnvUnchangedAndErrors(t *testing.T) {
	repo := newRepo(sampleEnv())
	result, err := NewEditEnv(repo, editTo(sampleEnv()), later).Execute(context.Background(), "a")
	if err != nil || result.Changed || repo.updates != 0 {
		t.Fatalf("result = %+v, %v, updates %d", result, err, repo.updates)
	}
	failing := &fakeEditor{result: func(domain.Env) (domain.EditResult, error) {
		return domain.EditResult{}, domain.ErrEditCanceled
	}}
	if _, err := NewEditEnv(repo, failing, later).Execute(context.Background(), "a"); !errors.Is(err, domain.ErrEditCanceled) {
		t.Fatalf("err = %v", err)
	}
	if _, err := NewEditEnv(repo, failing, later).Execute(context.Background(), "b"); !errors.Is(err, domain.ErrEnvNotFound) || failing.calls != 1 {
		t.Fatalf("missing err = %v, calls %d", err, failing.calls)
	}
}

func TestEditEnvDetectsConcurrentChange(t *testing.T) {
	repo := newRepo(sampleEnv())
	concurrent := &fakeEditor{result: func(initial domain.Env) (domain.EditResult, error) {
		if _, err := repo.snapshot.Modify("a", baseTime, func(env *domain.Env) error {
			env.Set("X", "1")
			return nil
		}); err != nil {
			return domain.EditResult{}, err
		}
		edited := initial.Clone()
		edited.Vars = []domain.Var{{Key: "Z", Value: "z"}}
		return domain.EditResult{Env: edited, Changed: true}, nil
	}}
	_, err := NewEditEnv(repo, concurrent, later).Execute(context.Background(), "a")
	if !errors.Is(err, domain.ErrEditConflict) || !strings.Contains(err.Error(), "mudou") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := repo.env(t, "a").Lookup("X"); !ok {
		t.Fatal("concurrent change lost")
	}
}
