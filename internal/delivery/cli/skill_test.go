package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/skill/domain"
)

const skillTestSource = "../../../skill/envault/SKILL.md"

func skillTestReadSource(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(skillTestSource)
	if err != nil {
		t.Fatalf("ler fonte: %v", err)
	}
	return raw
}

func skillTestAssertInstalled(t *testing.T, path string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ler instalado: %v", err)
	}
	if !bytes.Equal(got, skillTestReadSource(t)) {
		t.Fatal("arquivo instalado difere de skill/envault/SKILL.md")
	}
}

func TestSkillInstallCustomDir(t *testing.T) {
	ta := newTestApp(t)
	dir := filepath.Join(t.TempDir(), "skills")
	if code := ta.runWith(newSkillCmd(ta.App), "skill", "install", "--dir", dir); code != ExitOK {
		t.Fatalf("code = %d (%s)", code, ta.Err.String())
	}
	target := filepath.Join(dir, "envault", "SKILL.md")
	skillTestAssertInstalled(t, target)
	if ta.Out.Len() != 0 {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
	stderr := ta.Err.String()
	for _, want := range []string{target, `"permissions"`, "Bash(envault list:*)", "Bash(envault load:*)", "Read(./.env.local)", ".env.example"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr sem %q:\n%s", want, stderr)
		}
	}
	permissions, err := domain.PermissionsJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(stderr, permissions) {
		t.Errorf("stderr não termina com o bloco de permissões:\n%s", stderr)
	}
}

func TestSkillInstallDefaultDir(t *testing.T) {
	ta := newTestApp(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if code := ta.runWith(newSkillCmd(ta.App), "skill", "install"); code != ExitOK {
		t.Fatalf("code = %d (%s)", code, ta.Err.String())
	}
	skillTestAssertInstalled(t, filepath.Join(home, ".claude", "skills", "envault", "SKILL.md"))
}

func TestSkillInstallOverwritesOld(t *testing.T) {
	ta := newTestApp(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "envault", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("versão antiga"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := ta.runWith(newSkillCmd(ta.App), "skill", "install", "--dir", dir); code != ExitOK {
		t.Fatalf("code = %d (%s)", code, ta.Err.String())
	}
	skillTestAssertInstalled(t, target)
}

func TestSkillUsageErrors(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.runWith(newSkillCmd(ta.App), "skill", "install", "extra"); code != ExitUsage {
		t.Fatalf("code = %d, want %d", code, ExitUsage)
	}
	if code := ta.runWith(newSkillCmd(ta.App), "skill", "install", "--nope"); code != ExitUsage {
		t.Fatalf("code = %d, want %d", code, ExitUsage)
	}
}

func TestSkillWithoutSubcommandShowsHelp(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.runWith(newSkillCmd(ta.App), "skill"); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(ta.Out.String(), "install") {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}
