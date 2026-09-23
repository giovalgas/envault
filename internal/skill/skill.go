package skill

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	Name     = "envault"
	FileName = "SKILL.md"
	dirPerm  = 0o755
	filePerm = 0o644
)

var ErrNoHomeDir = errors.New("diretório home do usuário não encontrado")

type Permissions struct {
	Allow []string `json:"allow"`
	Ask   []string `json:"ask"`
	Deny  []string `json:"deny"`
}

type permissionsDocument struct {
	Permissions Permissions `json:"permissions"`
}

func Content() string {
	return content
}

func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNoHomeDir, err)
	}
	if home == "" {
		return "", ErrNoHomeDir
	}
	return filepath.Join(home, ".claude", "skills"), nil
}

func Path(dir string) string {
	return filepath.Join(dir, Name, FileName)
}

func Install(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("diretório de skills vazio")
	}
	target := Path(dir)
	if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
		return "", fmt.Errorf("criar diretório da skill: %w", err)
	}
	if err := writeReplacing(target, []byte(content)); err != nil {
		return "", err
	}
	return target, nil
}

func SuggestedPermissions() Permissions {
	return Permissions{
		Allow: []string{
			"Bash(envault --version)",
			"Bash(envault list:*)",
			"Bash(envault show:*)",
			"Bash(envault plan:*)",
		},
		Ask: []string{
			"Bash(envault load:*)",
			"Bash(envault exec:*)",
		},
		Deny: []string{
			"Bash(envault get:*)",
			"Bash(envault shell:*)",
			"Read(./.env)",
			"Read(./.env.local)",
		},
	}
}

func PermissionsJSON() (string, error) {
	encoded, err := json.MarshalIndent(permissionsDocument{Permissions: SuggestedPermissions()}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serializar permissões: %w", err)
	}
	return string(encoded) + "\n", nil
}

func writeReplacing(target string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+FileName+"-*")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário da skill: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("gravar skill: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("fechar skill: %w", err)
	}
	if err := os.Chmod(tmpName, filePerm); err != nil {
		cleanup()
		return fmt.Errorf("ajustar permissão da skill: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		cleanup()
		return fmt.Errorf("instalar skill em %s: %w", target, err)
	}
	return nil
}
