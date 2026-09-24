package gitignore

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/giovalgas/envault/internal/compose/domain"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git indisponível")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func initRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput()
	if err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestGitignoreIgnored(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), ".env\n")
	if got := IsIgnored(filepath.Join(repo, ".env")); got != domain.GitignoreIgnored {
		t.Fatalf("IsIgnored = %v", got)
	}
}

func TestGitignoreIgnoredWithoutFile(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), ".env*\n!.env.example\n")
	target := filepath.Join(repo, ".env.local")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target exists: %v", err)
	}
	if got := IsIgnored(target); got != domain.GitignoreIgnored {
		t.Fatalf("IsIgnored(.env.local) = %v", got)
	}
	if got := IsIgnored(filepath.Join(repo, ".env.example")); got != domain.GitignoreNotIgnored {
		t.Fatalf("IsIgnored(.env.example) = %v", got)
	}
}

func TestGitignoreNotIgnored(t *testing.T) {
	repo := initRepo(t)
	if got := IsIgnored(filepath.Join(repo, ".env")); got != domain.GitignoreNotIgnored {
		t.Fatalf("IsIgnored = %v", got)
	}
}

func TestGitignoreSubdirectory(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), "*.env\n")
	writeFile(t, filepath.Join(repo, "app", "keep"), "")
	if got := IsIgnored(filepath.Join(repo, "app", "prod.env")); got != domain.GitignoreIgnored {
		t.Fatalf("IsIgnored = %v", got)
	}
}

func TestGitignoreRelativePath(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), ".env\n")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Errorf("restore wd: %v", err)
		}
	})
	if got := IsIgnored(".env"); got != domain.GitignoreIgnored {
		t.Fatalf("IsIgnored = %v", got)
	}
}

func TestGitignoreOutsideRepoIsUnknown(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	if got := IsIgnored(filepath.Join(dir, ".env")); got != domain.GitignoreUnknown {
		t.Fatalf("IsIgnored = %v", got)
	}
}

func TestGitignoreMissingDirectoryIsUnknown(t *testing.T) {
	repo := initRepo(t)
	if got := IsIgnored(filepath.Join(repo, "nao", "existe", ".env")); got != domain.GitignoreUnknown {
		t.Fatalf("IsIgnored = %v", got)
	}
}

func TestGitignoreWithoutGitIsUnknown(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got := IsIgnored(filepath.Join(t.TempDir(), ".env")); got != domain.GitignoreUnknown {
		t.Fatalf("IsIgnored = %v", got)
	}
}

func TestGitignoreChecker(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), ".env\n")
	if got := New().IsIgnored(filepath.Join(repo, ".env")); got != domain.GitignoreIgnored {
		t.Fatalf("Checker.IsIgnored = %v", got)
	}
}
