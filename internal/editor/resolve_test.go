package editor

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func editorLookup(values map[string]string) LookupFunc {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

func TestResolveEditorCommand(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		goos     string
		wantName string
		wantArgs []string
		warns    bool
	}{
		{"visual vence editor", map[string]string{EnvVisual: "nvim", EnvEditor: "nano"}, "linux", "nvim", []string{"-n"}, false},
		{"editor sozinho", map[string]string{EnvEditor: "nano"}, "linux", "nano", nil, false},
		{"visual em branco cai no editor", map[string]string{EnvVisual: "  ", EnvEditor: "nano"}, "linux", "nano", nil, false},
		{"padrão unix", nil, "darwin", "vi", nil, false},
		{"padrão windows", nil, "windows", "notepad", nil, false},
		{"code com wait preservado", map[string]string{EnvEditor: "code --wait"}, "linux", "code", []string{"--wait"}, false},
		{"code sem wait", map[string]string{EnvEditor: "code"}, "linux", "code", []string{"--wait"}, true},
		{"code com -w", map[string]string{EnvEditor: "code -w"}, "linux", "code", []string{"-w"}, false},
		{"subl sem wait", map[string]string{EnvEditor: "subl"}, "linux", "subl", []string{"--wait"}, true},
		{"zed sem wait", map[string]string{EnvEditor: "zed"}, "linux", "zed", []string{"--wait"}, true},
		{"codium sem wait", map[string]string{EnvEditor: "/usr/bin/codium --new-window"}, "linux", "/usr/bin/codium", []string{"--new-window", "--wait"}, true},
		{"vim sem swap nem viminfo", map[string]string{EnvEditor: "vim"}, "linux", "vim", []string{"-n", "-i", "NONE"}, false},
		{"vim com caminho", map[string]string{EnvEditor: "/usr/local/bin/vim -u NONE"}, "linux", "/usr/local/bin/vim", []string{"-u", "NONE", "-n", "-i", "NONE"}, false},
		{"nvim só sem swap", map[string]string{EnvEditor: "nvim"}, "linux", "nvim", []string{"-n"}, false},
		{"aspas simples", map[string]string{EnvEditor: `'/opt/my editor/bin/ed' -x`}, "linux", "/opt/my editor/bin/ed", []string{"-x"}, false},
		{
			"windows com barra invertida",
			map[string]string{EnvEditor: `"C:\Program Files\Microsoft VS Code\bin\code.cmd"`},
			"windows", `C:\Program Files\Microsoft VS Code\bin\code.cmd`, []string{"--wait"}, true,
		},
		{"windows vim exe", map[string]string{EnvEditor: `C:\tools\vim.exe`}, "windows", `C:\tools\vim.exe`, []string{"-n", "-i", "NONE"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := Resolve(editorLookup(tc.env), tc.goos)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if spec.Name != tc.wantName || !slices.Equal(spec.Args, tc.wantArgs) {
				t.Fatalf("spec = %q %q, want %q %q", spec.Name, spec.Args, tc.wantName, tc.wantArgs)
			}
			if (len(spec.Warnings) > 0) != tc.warns {
				t.Fatalf("warnings = %q, want warns=%v", spec.Warnings, tc.warns)
			}
			if tc.warns && !strings.Contains(spec.Warnings[0], waitFlag) {
				t.Fatalf("warning does not mention %s: %q", waitFlag, spec.Warnings[0])
			}
		})
	}
}

func TestResolveEditorRejectsInvalid(t *testing.T) {
	for _, raw := range []string{`"code`, `vim; rm -rf /`, `""`, "vim | cat"} {
		_, err := Resolve(editorLookup(map[string]string{EnvEditor: raw}), "linux")
		if !errors.Is(err, ErrNoEditor) {
			t.Errorf("%q: err = %v, want ErrNoEditor", raw, err)
		}
	}
}

func TestEditorSpecArgv(t *testing.T) {
	spec := Spec{Name: "code", Args: []string{"--wait"}}
	first := spec.Argv("/tmp/a.env")
	second := spec.Argv("/tmp/b.env")
	if !slices.Equal(first, []string{"code", "--wait", "/tmp/a.env"}) || second[2] != "/tmp/b.env" {
		t.Fatalf("argv = %q %q", first, second)
	}
}
