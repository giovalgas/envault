package usecase

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/giovalgas/envault/internal/shared/dotenv"
)

func TestRenderEnvFile(t *testing.T) {
	reader := newReader(
		env("a", "X", "1", "QUOTE", "it's", "MULTI", "l1\nl2"),
		env("b", "X", "2", "EMPTY", ""),
	)
	got, err := NewRenderEnvFile(reader).Execute(context.Background(), RenderEnvFileInput{Envs: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	want := string(dotenv.Format(dotenv.Document{Vars: []dotenv.Var{
		{Key: "X", Value: "2"},
		{Key: "QUOTE", Value: "it's"},
		{Key: "MULTI", Value: "l1\nl2"},
		{Key: "EMPTY", Value: ""},
	}}))
	if got.Content != want {
		t.Fatalf("content:\n%s\nwant\n%s", got.Content, want)
	}
	parsed, err := dotenv.Parse([]byte(got.Content))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(parsed.Vars) != 4 || parsed.Vars[0].Value != "2" || parsed.Vars[2].Value != "l1\nl2" {
		t.Fatalf("parsed = %+v", parsed.Vars)
	}
	if !slices.Equal(got.Plan.Conflicts, []string{"X"}) || !slices.Equal(got.Plan.Envs, []string{"a", "b"}) {
		t.Fatalf("plan = %+v", got.Plan)
	}
}

func TestRenderEnvFileTemplate(t *testing.T) {
	reader := newReader(env("a", "A", "1", "EXTRA", "x"))
	tmpl := TemplateView{Found: true, Path: ".env.example", Entries: []TemplateEntryView{
		{Key: "A"},
		{Key: "PORT", Default: "8080", HasDefault: true},
		{Key: "MISSING"},
	}}
	got, err := NewRenderEnvFile(reader).Execute(context.Background(), RenderEnvFileInput{Envs: []string{"a"}, Template: tmpl, OnlyTemplate: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "A=1\nPORT=8080\n" {
		t.Fatalf("content = %q", got.Content)
	}
	if !slices.Equal(got.Plan.Missing, []string{"MISSING"}) {
		t.Fatalf("missing = %v", got.Plan.Missing)
	}
}

func TestRenderEnvFileErrors(t *testing.T) {
	if _, err := NewRenderEnvFile(&fakeReader{err: errBoom}).Execute(context.Background(), RenderEnvFileInput{Envs: []string{"a"}}); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	var dup *DuplicateEnvError
	if _, err := NewRenderEnvFile(newReader()).Execute(context.Background(), RenderEnvFileInput{Envs: []string{"a", "a"}}); !errors.As(err, &dup) {
		t.Fatalf("err = %v", err)
	}
}
