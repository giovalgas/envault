package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	EnvHome       = "ENVAULT_HOME"
	EnvKey        = "ENVAULT_KEY"
	EnvDebug      = "ENVAULT_DEBUG"
	EnvRuntimeDir = "XDG_RUNTIME_DIR"

	AppDirName    = "envault"
	VaultFileName = "vault.enc"
	KeyFileName   = "key"
	LockFileName  = "vault.lock"

	SelectionFileName = "selection.json"

	DirPerm  os.FileMode = 0o700
	FilePerm os.FileMode = 0o600
)

var ErrNoConfigDir = errors.New("diretório de configuração do usuário indisponível")

type Lookup func(key string) (string, bool)

type UserConfigDir func() (string, error)

type Paths struct {
	Dir   string
	Vault string
	Key   string
	Lock  string
}

type Config struct {
	Paths       Paths
	KeyOverride string
	Debug       bool
	RuntimeDir  string
}

func PathsIn(dir string) Paths {
	return Paths{
		Dir:   dir,
		Vault: filepath.Join(dir, VaultFileName),
		Key:   filepath.Join(dir, KeyFileName),
		Lock:  filepath.Join(dir, LockFileName),
	}
}

func Load(lookup Lookup, userConfigDir UserConfigDir) (Config, error) {
	dir, err := resolveDir(lookup, userConfigDir)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Paths:       PathsIn(dir),
		KeyOverride: strings.TrimSpace(value(lookup, EnvKey)),
		Debug:       isTruthy(value(lookup, EnvDebug)),
		RuntimeDir:  value(lookup, EnvRuntimeDir),
	}, nil
}

func FromOS() (Config, error) {
	return Load(os.LookupEnv, os.UserConfigDir)
}

func resolveDir(lookup Lookup, userConfigDir UserConfigDir) (string, error) {
	if home := value(lookup, EnvHome); home != "" {
		return filepath.Clean(home), nil
	}
	base, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNoConfigDir, err)
	}
	if base == "" {
		return "", ErrNoConfigDir
	}
	return filepath.Join(base, AppDirName), nil
}

func value(lookup Lookup, key string) string {
	v, ok := lookup(key)
	if !ok {
		return ""
	}
	return v
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
