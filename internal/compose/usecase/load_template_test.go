package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadTemplateDefaultFound(t *testing.T) {
	files := &fakeTemplates{content: map[string]string{DefaultTemplateFile: "X=\nPORT=3000\n"}}
	tmpl, err := NewLoadTemplate(files).Execute(context.Background(), LoadTemplateInput{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if tmpl.Path != DefaultTemplateFile || !tmpl.Found || !slices.Equal(tmpl.Keys(), []string{"X", "PORT"}) {
		t.Fatalf("template = %+v", tmpl)
	}
	if want := (TemplateEntryView{Key: "PORT", Default: "3000", HasDefault: true}); tmpl.Entries[1] != want {
		t.Fatalf("entry = %+v", tmpl.Entries[1])
	}
}

func TestLoadTemplateDefaultMissing(t *testing.T) {
	tmpl, err := NewLoadTemplate(&fakeTemplates{}).Execute(context.Background(), LoadTemplateInput{})
	if err != nil || tmpl.Path != DefaultTemplateFile || tmpl.Found || tmpl.toDomain() != nil {
		t.Fatalf("template = %+v, %v", tmpl, err)
	}
	if _, err := NewLoadTemplate(&fakeTemplates{}).Execute(context.Background(), LoadTemplateInput{Only: true}); !errors.Is(err, ErrTemplateRequired) {
		t.Fatalf("only err = %v", err)
	}
}

func TestLoadTemplateExplicitMissing(t *testing.T) {
	_, err := NewLoadTemplate(&fakeTemplates{}).Execute(context.Background(), LoadTemplateInput{Path: "t.env"})
	if !errors.Is(err, ErrTemplateNotFound) || !strings.Contains(err.Error(), "t.env") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadTemplateDisabledReadsNothing(t *testing.T) {
	files := &fakeTemplates{}
	tmpl, err := NewLoadTemplate(files).Execute(context.Background(), LoadTemplateInput{Disabled: true})
	if err != nil || tmpl.Path != "" || tmpl.Found || len(files.paths) != 0 {
		t.Fatalf("template = %+v, %v, paths %v", tmpl, err, files.paths)
	}
}

func TestLoadTemplateResolvesDefaultInDir(t *testing.T) {
	dir := t.TempDir()
	files := &fakeTemplates{content: map[string]string{filepath.Join(dir, DefaultTemplateFile): "X=\n"}}
	tmpl, err := NewLoadTemplate(files).Execute(context.Background(), LoadTemplateInput{Dir: dir})
	if err != nil || !tmpl.Found || tmpl.Path != DefaultTemplateFile {
		t.Fatalf("template = %+v, %v", tmpl, err)
	}
}

func TestLoadTemplateErrors(t *testing.T) {
	_, err := NewLoadTemplate(&fakeTemplates{err: errBoom}).Execute(context.Background(), LoadTemplateInput{})
	var readErr *TemplateReadError
	if !errors.As(err, &readErr) || !errors.Is(err, errBoom) || err.Error() != "ler template .env.example: boom" {
		t.Fatalf("read err = %v", err)
	}
	files := &fakeTemplates{content: map[string]string{"t.env": "1BAD=\n"}}
	_, err = NewLoadTemplate(files).Execute(context.Background(), LoadTemplateInput{Path: "t.env"})
	var parseErr *TemplateParseError
	if !errors.As(err, &parseErr) || !errors.Is(err, ErrSyntax) || !strings.HasPrefix(err.Error(), "template t.env: ") {
		t.Fatalf("parse err = %v", err)
	}
}
