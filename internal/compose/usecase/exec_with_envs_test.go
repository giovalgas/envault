package usecase

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"testing"

	"github.com/giovalgas/envault/internal/compose/domain"
)

func TestExecEnviron(t *testing.T) {
	base := []string{"PATH=/bin", "X=old", "KEEP=1", "=C:=C:\\"}
	got := overlayEnviron(base, []domain.Var{{Key: "X", Value: "new"}, {Key: "Y", Value: "a=b"}})
	want := []string{"PATH=/bin", "KEEP=1", "=C:=C:\\", "X=new", "Y=a=b"}
	if !slices.Equal(got, want) {
		t.Fatalf("environ = %v", got)
	}
	if runtime.GOOS == "windows" {
		got = overlayEnviron([]string{"Path=C:\\"}, []domain.Var{{Key: "PATH", Value: "D:\\"}})
		if !slices.Equal(got, []string{"PATH=D:\\"}) {
			t.Fatalf("windows environ = %v", got)
		}
	}
}

func TestExecWithEnvs(t *testing.T) {
	reader := newReader(env("a", "X", "1"), env("b", "X", "2", "Y", "3"))
	tmpl := TemplateView{Found: true, Entries: []TemplateEntryView{{Key: "X"}, {Key: "SENTRY_DSN"}}}
	environment := fakeEnvironment{"X=processo", "KEEP=1"}
	result, err := NewExecWithEnvs(reader, environment).Execute(context.Background(), ExecWithEnvsInput{
		Envs:         []string{"a", "b"},
		Template:     tmpl,
		OnlyTemplate: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !slices.Equal(result.Environ, []string{"KEEP=1", "X=2"}) {
		t.Fatalf("environ = %v", result.Environ)
	}
	if !slices.Equal(result.Plan.Missing, []string{"SENTRY_DSN"}) {
		t.Fatalf("missing = %v", result.Plan.Missing)
	}
	if _, err := NewExecWithEnvs(&fakeReader{err: errBoom}, fakeEnvironment{}).Execute(context.Background(), ExecWithEnvsInput{Envs: []string{"a"}}); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
}
