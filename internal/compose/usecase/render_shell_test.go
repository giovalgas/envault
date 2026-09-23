package usecase

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestShellRender(t *testing.T) {
	reader := newReader(
		env("a", "X", "1", "QUOTE", "it's", "DOLLAR", "$HOME `x` \\n", "MULTI", "l1\nl2"),
		env("b", "X", "2", "EMPTY", ""),
	)
	posix := "export X='2'\n" +
		"export QUOTE='it'\\''s'\n" +
		"export DOLLAR='$HOME `x` \\n'\n" +
		"export MULTI='l1\nl2'\n" +
		"export EMPTY=''\n"
	fish := "set -gx X '2'\n" +
		"set -gx QUOTE 'it\\'s'\n" +
		"set -gx DOLLAR '$HOME `x` \\\\n'\n" +
		"set -gx MULTI 'l1\nl2'\n" +
		"set -gx EMPTY ''\n"
	for dialect, want := range map[string]string{ShellBash: posix, ShellZsh: posix, ShellFish: fish} {
		got, err := NewRenderShell(reader).Execute(context.Background(), RenderShellInput{Envs: []string{"a", "b"}, Dialect: dialect})
		if err != nil {
			t.Fatalf("%s: %v", dialect, err)
		}
		if got != want {
			t.Fatalf("%s:\n%s\nwant\n%s", dialect, got, want)
		}
	}
	if !slices.Equal(ShellDialects(), []string{ShellBash, ShellZsh, ShellFish}) {
		t.Fatalf("dialects = %v", ShellDialects())
	}
}

func TestShellRenderErrors(t *testing.T) {
	if _, err := NewRenderShell(&fakeReader{err: errBoom}).Execute(context.Background(), RenderShellInput{Envs: []string{"a"}}); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
}
