package cli

import (
	"path/filepath"
	"testing"
)

func shellTestSeed(t *testing.T, ta *testApp) {
	t.Helper()
	ta.seed(t,
		planTestEnv("a", "X", "1", "QUOTE", "it's", "DOLLAR", "$HOME `x` \\n", "MULTI", "l1\nl2"),
		planTestEnv("b", "X", "2", "EMPTY", ""),
	)
}

func TestShellDialects(t *testing.T) {
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
	cases := map[string]string{shellBash: posix, shellZsh: posix, shellFish: fish}
	for dialect, want := range cases {
		t.Run(dialect, func(t *testing.T) {
			planTestWorkdir(t)
			ta := newTestApp(t)
			shellTestSeed(t, ta)
			if code := ta.runWith(newShellCmd(ta.App), "shell", "a", "b", "--shell", dialect); code != ExitOK {
				t.Fatalf("code = %d stderr %s", code, ta.Err.String())
			}
			if got := ta.Out.String(); got != want {
				t.Fatalf("stdout =\n%s\nwant\n%s", got, want)
			}
			if ta.Err.Len() != 0 {
				t.Fatalf("stderr = %q", ta.Err.String())
			}
		})
	}
}

func TestShellDefaultDialect(t *testing.T) {
	cases := []struct {
		shell string
		want  string
	}{
		{shell: "/usr/local/bin/fish", want: "set -gx X '1'\n"},
		{shell: "/bin/zsh", want: "export X='1'\n"},
		{shell: filepath.Join("C:", "tools", "fish.exe"), want: "set -gx X '1'\n"},
		{shell: "/bin/tcsh", want: "export X='1'\n"},
		{shell: "", want: "export X='1'\n"},
	}
	for _, tc := range cases {
		planTestWorkdir(t)
		t.Setenv(shellEnvVar, tc.shell)
		ta := newTestApp(t)
		ta.seed(t, planTestEnv("a", "X", "1"))
		if code := ta.runWith(newShellCmd(ta.App), "shell", "a"); code != ExitOK {
			t.Fatalf("%q: code = %d", tc.shell, code)
		}
		if got := ta.Out.String(); got != tc.want {
			t.Fatalf("%q: stdout = %q", tc.shell, got)
		}
	}
}

func TestShellIgnoresTemplate(t *testing.T) {
	dir := planTestWorkdir(t)
	planTestWrite(t, filepath.Join(dir, templateDefaultPath), "PORT=3000\n")
	ta := newTestApp(t)
	ta.seed(t, planTestEnv("a", "X", "1"))
	if code := ta.runWith(newShellCmd(ta.App), "shell", "a", "--shell", shellBash); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	if got := ta.Out.String(); got != "export X='1'\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestShellErrors(t *testing.T) {
	planTestWorkdir(t)
	ta := newTestApp(t)
	shellTestSeed(t, ta)
	cases := []struct {
		name string
		args []string
		code int
	}{
		{name: "unsupported shell", args: []string{"shell", "a", "--shell", "powershell"}, code: ExitUsage},
		{name: "empty shell", args: []string{"shell", "a", "--shell", ""}, code: ExitUsage},
		{name: "no env", args: []string{"shell"}, code: ExitUsage},
		{name: "unknown env", args: []string{"shell", "nao-existe"}, code: ExitEnvNotFound},
	}
	for _, tc := range cases {
		if code := ta.runWith(newShellCmd(ta.App), tc.args...); code != tc.code {
			t.Errorf("%s: code = %d, want %d", tc.name, code, tc.code)
		}
		if ta.Out.Len() != 0 {
			t.Errorf("%s: stdout = %q", tc.name, ta.Out.String())
		}
	}
}
