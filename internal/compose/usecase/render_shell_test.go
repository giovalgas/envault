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
		if got.Script != want {
			t.Fatalf("%s:\n%s\nwant\n%s", dialect, got.Script, want)
		}
		if !slices.Equal(got.Plan.Conflicts, []string{"X"}) {
			t.Fatalf("%s: conflicts = %v", dialect, got.Plan.Conflicts)
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

func TestDetectShell(t *testing.T) {
	cases := map[string]string{
		"/bin/zsh":            ShellZsh,
		"/usr/local/bin/fish": ShellFish,
		"/opt/zsh.exe":        ShellZsh,
		"/bin/tcsh":           ShellBash,
		"":                    ShellBash,
	}
	for shellPath, want := range cases {
		if got := DetectShell(shellPath); got != want {
			t.Errorf("DetectShell(%q) = %q, want %q", shellPath, got, want)
		}
	}
}
