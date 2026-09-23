package usecase

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/giovalgas/envault/internal/compose/domain"
)

func TestPlanLoadResolvesAndInspectsTarget(t *testing.T) {
	reader := newReader(env("a", "X", "1", "A", "1"), env("b", "X", "2"))
	files := &fakeFiles{exists: true}
	ignore := &fakeGitignore{status: domain.GitignoreIgnored}
	tmpl := &domain.Template{Entries: []domain.TemplateEntry{{Key: "X"}, {Key: "PORT", Default: "3000", HasDefault: true}}}
	result, err := NewPlanLoad(reader, files, ignore).Execute(context.Background(), PlanLoadInput{
		Envs:     []string{"a", "b"},
		Template: tmpl,
		Options:  domain.Options{OnlyTemplate: true},
		Target:   ".env",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !slices.Equal(result.Plan.Pairs(), vars("X", "2", "PORT", "3000")) {
		t.Fatalf("pairs = %+v", result.Plan.Pairs())
	}
	if !slices.Equal(result.Plan.Conflicts, []string{"X"}) || !slices.Equal(result.Plan.Extra, []string{"A"}) {
		t.Fatalf("plan = %+v", result.Plan)
	}
	want := domain.Target{Path: ".env", Exists: true, Gitignored: domain.GitignoreIgnored}
	if result.Target != want || !slices.Equal(ignore.paths, []string{".env"}) {
		t.Fatalf("target = %+v paths %v", result.Target, ignore.paths)
	}
	if files.writes != 0 || files.merges != 0 {
		t.Fatal("plan wrote the target")
	}
}

func TestPlanLoadDuplicateEnvBeforeReading(t *testing.T) {
	reader := newReader(env("a"))
	_, err := NewPlanLoad(reader, &fakeFiles{}, &fakeGitignore{}).Execute(context.Background(), PlanLoadInput{Envs: []string{"a", "b", "a"}})
	var duplicate *DuplicateEnvError
	if !errors.As(err, &duplicate) || duplicate.Name != "a" {
		t.Fatalf("err = %v", err)
	}
	if err.Error() != `env "a" repetida` {
		t.Fatalf("message = %q", err.Error())
	}
	if reader.calls != 0 {
		t.Fatalf("reader called %d times", reader.calls)
	}
}

func TestPlanLoadErrors(t *testing.T) {
	reader := &fakeReader{err: errBoom}
	if _, err := NewPlanLoad(reader, &fakeFiles{}, &fakeGitignore{}).Execute(context.Background(), PlanLoadInput{Envs: []string{"a"}}); !errors.Is(err, errBoom) {
		t.Fatalf("reader err = %v", err)
	}
	files := &fakeFiles{existsErr: errBoom}
	ignore := &fakeGitignore{}
	if _, err := NewPlanLoad(newReader(env("a")), files, ignore).Execute(context.Background(), PlanLoadInput{Envs: []string{"a"}}); !errors.Is(err, errBoom) {
		t.Fatalf("exists err = %v", err)
	}
	if len(ignore.paths) != 0 {
		t.Fatalf("gitignore checked after error: %v", ignore.paths)
	}
}

func TestPlanLoadEnvNotFoundMessage(t *testing.T) {
	_, err := NewPlanLoad(newReader(), &fakeFiles{}, &fakeGitignore{}).Execute(context.Background(), PlanLoadInput{Envs: []string{"x"}})
	var missing *EnvNotFoundError
	if !errors.As(err, &missing) || missing.Name != "x" || err.Error() != `env "x" não encontrada` {
		t.Fatalf("err = %v", err)
	}
}
