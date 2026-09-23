package cli

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/giovalgas/envault/internal/config"
	"github.com/giovalgas/envault/internal/store"
	"github.com/giovalgas/envault/internal/vault"
)

type App struct {
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Version    string
	LoadConfig func() (config.Config, error)
	KeyStore   func(cfg config.Config) store.KeyStore
	IsTerminal func(stream any) bool
}

func NewApp(version string) *App {
	return &App{
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Version:    version,
		LoadConfig: config.FromOS,
		KeyStore:   FileKeyStore,
		IsTerminal: isTerminal,
	}
}

func FileKeyStore(cfg config.Config) store.KeyStore {
	return store.NewFile(cfg.Paths.Key)
}

func (a *App) Config() (config.Config, error) {
	cfg, err := a.LoadConfig()
	if err != nil {
		return config.Config{}, fmt.Errorf("carregar configuração: %w", err)
	}
	return cfg, nil
}

func (a *App) OpenVault() (*vault.Vault, error) {
	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}
	keys, err := store.WithOverride(cfg.KeyOverride, a.KeyStore(cfg))
	if err != nil {
		return nil, err
	}
	return vault.New(cfg.Paths, keys), nil
}

func (a *App) StdinIsTerminal() bool {
	return a.IsTerminal(a.Stdin)
}

func (a *App) StdoutIsTerminal() bool {
	return a.IsTerminal(a.Stdout)
}

func (a *App) Interactive() bool {
	return a.StdinIsTerminal() && a.StdoutIsTerminal()
}

func (a *App) Infof(format string, args ...any) {
	fmt.Fprintf(a.Stderr, format+"\n", args...)
}

func isTerminal(stream any) bool {
	file, ok := stream.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}
