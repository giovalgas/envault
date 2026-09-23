package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestShellWrapperScripts(t *testing.T) {
	for _, dialect := range ShellDialects() {
		script, err := NewRenderShellWrapper().Execute(context.Background(), dialect)
		if err != nil {
			t.Fatalf("%s: %v", dialect, err)
		}
		for _, want := range []string{ExportFileVar + "=", ExportShellVar + "=" + dialect + " command envault", "mktemp -d", "rmdir", "load"} {
			if !strings.Contains(script, want) {
				t.Errorf("%s: script sem %q:\n%s", dialect, want, script)
			}
		}
		for i, line := range strings.Split(script, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				t.Errorf("%s: linha %d é comentário: %q", dialect, i+1, line)
			}
		}
	}
}

func TestShellWrapperRejectsUnknownShell(t *testing.T) {
	if _, err := NewRenderShellWrapper().Execute(context.Background(), "tcsh"); !errors.Is(err, ErrUnsupportedShell) {
		t.Fatalf("err = %v", err)
	}
}

func TestShellInitLine(t *testing.T) {
	cases := map[string]string{
		ShellBash: `eval "$(envault shell-init bash)"`,
		ShellZsh:  `eval "$(envault shell-init zsh)"`,
		ShellFish: "envault shell-init fish | source",
		"tcsh":    `eval "$(envault shell-init zsh)"`,
	}
	for dialect, want := range cases {
		if got := ShellInitLine(dialect); got != want {
			t.Errorf("%s: %q", dialect, got)
		}
	}
}
