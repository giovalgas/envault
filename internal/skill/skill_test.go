package skill

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

const sourcePath = "../../skill/envault/SKILL.md"

func readSource(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("ler fonte: %v", err)
	}
	return raw
}

func TestSkillContentMatchesSource(t *testing.T) {
	if !bytes.Equal([]byte(Content()), readSource(t)) {
		t.Fatal("content.go fora de sincronia com skill/envault/SKILL.md; rode make skill-sync")
	}
}

func TestSkillContentFrontmatter(t *testing.T) {
	c := Content()
	if !strings.HasPrefix(c, "---\nname: envault\ndescription: ") {
		t.Fatalf("frontmatter inesperado: %q", c[:min(len(c), 60)])
	}
	for _, want := range []string{".env.example", "segredos", "credenciais", "chaves de API"} {
		if !strings.Contains(c, want) {
			t.Errorf("description sem %q", want)
		}
	}
}

func TestSkillContentRules(t *testing.T) {
	c := Content()
	for _, want := range []string{
		"NUNCA rode `envault get`, `envault shell`",
		"NUNCA leia arquivos `.env`",
		"NUNCA rode `envault new`/`edit`",
		"NUNCA rode `envault load` sem confirmação explícita",
		"`envault --version`, `envault list`, `envault show` e `envault plan`",
		"| 3 | env não encontrada |",
		"| 7 | validação |",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("SKILL.md sem %q", want)
		}
	}
}

func TestSkillInstallCreatesDirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")
	path, err := Install(dir)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if path != filepath.Join(dir, "envault", "SKILL.md") {
		t.Fatalf("path = %q", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler instalado: %v", err)
	}
	if !bytes.Equal(got, readSource(t)) {
		t.Fatal("arquivo instalado difere da fonte")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != filePerm {
			t.Fatalf("perm = %o", info.Mode().Perm())
		}
	}
}

func TestSkillInstallOverwrites(t *testing.T) {
	dir := t.TempDir()
	target := Path(dir)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("conteúdo antigo"+strings.Repeat("x", 10000)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != Content() {
		t.Fatal("reinstalação não sobrescreveu")
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("sobrou arquivo temporário: %v", entries)
	}
}

func TestSkillInstallEmptyDir(t *testing.T) {
	if _, err := Install(""); err == nil {
		t.Fatal("Install vazio sem erro")
	}
}

func TestSkillDefaultDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	if dir != filepath.Join(home, ".claude", "skills") {
		t.Fatalf("dir = %q", dir)
	}
}

func TestSkillPermissionsJSON(t *testing.T) {
	raw, err := PermissionsJSON()
	if err != nil {
		t.Fatalf("PermissionsJSON: %v", err)
	}
	var doc permissionsDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	p := doc.Permissions
	if len(p.Allow) != 4 || len(p.Ask) != 2 || len(p.Deny) != 4 {
		t.Fatalf("permissões = %+v", p)
	}
	for _, want := range []string{"Read(./.env)", "Read(./.env.local)", "Bash(envault get:*)", "Bash(envault shell:*)"} {
		if !slices.Contains(p.Deny, want) {
			t.Errorf("deny sem %q", want)
		}
	}
	if strings.Contains(raw, ".env.example") || strings.Contains(raw, ".env.*") {
		t.Error(".env.example não pode ser bloqueado")
	}
}
