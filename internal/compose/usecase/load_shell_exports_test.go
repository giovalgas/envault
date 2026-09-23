package usecase

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/giovalgas/envault/internal/compose/domain"
)

func TestLoadShellExportsWritesDialect(t *testing.T) {
	reader := newReader(env("a", "X", "1", "Q", "it's"), env("b", "X", "2"))
	tmpl := &domain.Template{Entries: []domain.TemplateEntry{{Key: "X"}, {Key: "PORT", Default: "3000", HasDefault: true}, {Key: "MISSING"}}}
	cases := []struct {
		dialect string
		want    string
	}{
		{dialect: ShellZsh, want: "export X='2'\nexport PORT='3000'\nexport Q='it'\\''s'\n"},
		{dialect: ShellFish, want: "set -gx X '2'\nset -gx PORT '3000'\nset -gx Q 'it\\'s'\n"},
	}
	for _, tc := range cases {
		exports := &fakeExports{}
		result, err := NewLoadShellExports(NewRenderShell(reader), exports).Execute(context.Background(), LoadShellExportsInput{
			Envs:       []string{"a", "b"},
			Template:   tmpl,
			Dialect:    tc.dialect,
			ExportFile: "/tmp/exports",
		})
		if err != nil {
			t.Fatalf("%s: %v", tc.dialect, err)
		}
		if exports.path != "/tmp/exports" || exports.script != tc.want {
			t.Fatalf("%s: path %q script %q", tc.dialect, exports.path, exports.script)
		}
		if result.Written != 3 || !slices.Equal(result.Plan.Missing, []string{"MISSING"}) || !slices.Equal(result.Plan.Conflicts, []string{"X"}) {
			t.Fatalf("%s: result = %+v", tc.dialect, result)
		}
	}
}

func TestLoadShellExportsOnlyTemplate(t *testing.T) {
	exports := &fakeExports{}
	tmpl := &domain.Template{Entries: []domain.TemplateEntry{{Key: "X"}}}
	_, err := NewLoadShellExports(NewRenderShell(newReader(env("a", "X", "1", "Y", "2"))), exports).Execute(context.Background(), LoadShellExportsInput{
		Envs:       []string{"a"},
		Template:   tmpl,
		Options:    domain.Options{OnlyTemplate: true},
		Dialect:    ShellBash,
		ExportFile: "exports",
	})
	if err != nil || exports.script != "export X='1'\n" {
		t.Fatalf("err %v script %q", err, exports.script)
	}
}

func TestLoadShellExportsErrors(t *testing.T) {
	reader := newReader(env("a", "X", "1"))
	exports := &fakeExports{}
	_, err := NewLoadShellExports(NewRenderShell(reader), exports).Execute(context.Background(), LoadShellExportsInput{Envs: []string{"a"}})
	if !errors.Is(err, ErrNoExportFile) || exports.writes != 0 || reader.calls != 0 {
		t.Fatalf("err %v writes %d reads %d", err, exports.writes, reader.calls)
	}
	_, err = NewLoadShellExports(NewRenderShell(reader), exports).Execute(context.Background(), LoadShellExportsInput{Envs: []string{"a", "x"}, ExportFile: "e"})
	var missing *EnvNotFoundError
	if !errors.As(err, &missing) || exports.writes != 0 {
		t.Fatalf("err %v writes %d", err, exports.writes)
	}
	failing := &fakeExports{err: errBoom}
	_, err = NewLoadShellExports(NewRenderShell(reader), failing).Execute(context.Background(), LoadShellExportsInput{Envs: []string{"a"}, ExportFile: "e"})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
}
