package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
)

const (
	Name     = "envault"
	FileName = "SKILL.md"
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

func Path(dir string) string {
	return filepath.Join(dir, Name, FileName)
}

func DefaultDir(home string) (string, error) {
	if home == "" {
		return "", ErrNoHomeDir
	}
	return filepath.Join(home, ".claude", "skills"), nil
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
