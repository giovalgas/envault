package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/atotto/clipboard"
	"golang.org/x/term"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	"github.com/giovalgas/envault/internal/shared/config"
	skillusecase "github.com/giovalgas/envault/internal/skill/usecase"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const shellEnvVar = "SHELL"

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type VaultUseCases struct {
	InitVault *vaultusecase.InitVault
	ListEnvs  *vaultusecase.ListEnvs
	ShowEnv   *vaultusecase.ShowEnv
	GetValue  *vaultusecase.GetValue
	SetValues *vaultusecase.SetValues
	UnsetKeys *vaultusecase.UnsetKeys
	CreateEnv *vaultusecase.CreateEnv
	EditEnv   *vaultusecase.EditEnv
	ImportEnv *vaultusecase.ImportEnv
	RenameEnv *vaultusecase.RenameEnv
	CopyEnv   *vaultusecase.CopyEnv
	DeleteEnv *vaultusecase.DeleteEnv

	BeginCreateEnv *vaultusecase.BeginCreateEnv
	BeginEditEnv   *vaultusecase.BeginEditEnv
}

type VaultDeps struct {
	Repository vaultusecase.EnvRepository
	Editor     vaultusecase.Editor
	Sessions   vaultusecase.SessionEditor
	Files      vaultusecase.EnvFileReader
	Clock      vaultusecase.Clock
}

func NewVaultUseCases(deps VaultDeps) VaultUseCases {
	repo, clock := deps.Repository, deps.Clock
	return VaultUseCases{
		InitVault: vaultusecase.NewInitVault(repo),
		ListEnvs:  vaultusecase.NewListEnvs(repo),
		ShowEnv:   vaultusecase.NewShowEnv(repo),
		GetValue:  vaultusecase.NewGetValue(repo),
		SetValues: vaultusecase.NewSetValues(repo, clock),
		UnsetKeys: vaultusecase.NewUnsetKeys(repo, clock),
		CreateEnv: vaultusecase.NewCreateEnv(repo, deps.Editor, deps.Files, clock),
		EditEnv:   vaultusecase.NewEditEnv(repo, deps.Editor, clock),
		ImportEnv: vaultusecase.NewImportEnv(repo, deps.Files, clock),
		RenameEnv: vaultusecase.NewRenameEnv(repo, clock),
		CopyEnv:   vaultusecase.NewCopyEnv(repo, clock),
		DeleteEnv: vaultusecase.NewDeleteEnv(repo),

		BeginCreateEnv: vaultusecase.NewBeginCreateEnv(repo, deps.Sessions, clock),
		BeginEditEnv:   vaultusecase.NewBeginEditEnv(repo, deps.Sessions, clock),
	}
}

type VaultOpener func() (VaultUseCases, error)

func (open VaultOpener) ListEnvs() (*vaultusecase.ListEnvs, error) {
	uc, err := open()
	if err != nil {
		return nil, err
	}
	return uc.ListEnvs, nil
}

type ConfigLoader func() (config.Config, error)

func (load ConfigLoader) Dir() (string, error) {
	cfg, err := load()
	if err != nil {
		return "", err
	}
	return cfg.Paths.Dir, nil
}

type ComposeUseCases struct {
	LoadTemplate       *composeusecase.LoadTemplate
	PlanLoad           *composeusecase.PlanLoad
	LoadEnvFile        *composeusecase.LoadEnvFile
	LoadShellExports   *composeusecase.LoadShellExports
	RenderEnvFile      *composeusecase.RenderEnvFile
	ExecWithEnvs       *composeusecase.ExecWithEnvs
	RunCommand         *composeusecase.RunCommand
	RenderShell        *composeusecase.RenderShell
	RenderShellWrapper *composeusecase.RenderShellWrapper
	GetSelection       *composeusecase.GetSelection
	SaveSelection      *composeusecase.SaveSelection
}

type ComposeDeps struct {
	Envs        composeusecase.EnvReader
	Catalog     composeusecase.EnvCatalog
	Files       composeusecase.TargetFiles
	Exports     composeusecase.ExportWriter
	Gitignore   composeusecase.GitignoreChecker
	Selections  composeusecase.SelectionStore
	Templates   composeusecase.TemplateReader
	Environment composeusecase.Environment
	Processes   composeusecase.ProcessRunner
}

func NewComposeUseCases(deps ComposeDeps) ComposeUseCases {
	render := composeusecase.NewRenderShell(deps.Envs)
	return ComposeUseCases{
		LoadTemplate:       composeusecase.NewLoadTemplate(deps.Templates),
		PlanLoad:           composeusecase.NewPlanLoad(deps.Envs, deps.Files, deps.Gitignore),
		LoadEnvFile:        composeusecase.NewLoadEnvFile(deps.Envs, deps.Files, deps.Gitignore),
		LoadShellExports:   composeusecase.NewLoadShellExports(render, deps.Exports),
		RenderEnvFile:      composeusecase.NewRenderEnvFile(deps.Envs),
		ExecWithEnvs:       composeusecase.NewExecWithEnvs(deps.Envs, deps.Environment),
		RunCommand:         composeusecase.NewRunCommand(deps.Processes),
		RenderShell:        render,
		RenderShellWrapper: composeusecase.NewRenderShellWrapper(),
		GetSelection:       composeusecase.NewGetSelection(deps.Catalog, deps.Selections),
		SaveSelection:      composeusecase.NewSaveSelection(deps.Selections, nil),
	}
}

type ShellExport struct {
	File    string
	Dialect string
}

func (e ShellExport) Active() bool {
	return e.File != ""
}

type Wiring struct {
	Vault        func(cfg config.Config, streams Streams) (VaultUseCases, error)
	KeyMigration func(cfg config.Config) *vaultusecase.MigrateKey
	Compose      func(cfg ConfigLoader, vault VaultOpener) ComposeUseCases
	Skill        func() *skillusecase.InstallSkill
	TUI          func(ctx context.Context, session TUISession) error
}

type TUISession struct {
	Vault   VaultUseCases
	Compose ComposeUseCases
	Export  ShellExport
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Debug   bool
}

type App struct {
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Version    string
	LoadConfig func() (config.Config, error)
	Wire       Wiring
	IsTerminal func(stream any) bool
	Clipboard  func(text string) error
}

func NewApp(version string, wire Wiring) *App {
	return &App{
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Version:    version,
		LoadConfig: config.FromOS,
		Wire:       wire,
		IsTerminal: isTerminal,
		Clipboard:  clipboard.WriteAll,
	}
}

func (a *App) Config() (config.Config, error) {
	cfg, err := a.LoadConfig()
	if err != nil {
		return config.Config{}, fmt.Errorf("carregar configuração: %w", err)
	}
	return cfg, nil
}

func (a *App) Vault() (VaultUseCases, error) {
	cfg, err := a.Config()
	if err != nil {
		return VaultUseCases{}, err
	}
	return a.Wire.Vault(cfg, Streams{Stdin: a.Stdin, Stdout: a.Stdout, Stderr: a.Stderr})
}

func (a *App) KeyMigration() (*vaultusecase.MigrateKey, error) {
	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}
	return a.Wire.KeyMigration(cfg), nil
}

func (a *App) Compose() ComposeUseCases {
	return a.Wire.Compose(a.Config, a.Vault)
}

func (a *App) TUISession() (TUISession, error) {
	cfg, err := a.Config()
	if err != nil {
		return TUISession{}, err
	}
	uc, err := a.Wire.Vault(cfg, Streams{Stdin: a.Stdin, Stdout: a.Stdout, Stderr: a.Stderr})
	if err != nil {
		return TUISession{}, err
	}
	loaded := func() (config.Config, error) { return cfg, nil }
	opener := func() (VaultUseCases, error) { return uc, nil }
	return TUISession{
		Vault:   uc,
		Compose: a.Wire.Compose(loaded, opener),
		Export:  a.ShellExport(),
		Stdin:   a.Stdin,
		Stdout:  a.Stdout,
		Stderr:  a.Stderr,
		Debug:   cfg.Debug,
	}, nil
}

func (a *App) ShellExport() ShellExport {
	dialect := os.Getenv(composeusecase.ExportShellVar)
	if !slices.Contains(composeusecase.ShellDialects(), dialect) {
		dialect = a.DetectShell()
	}
	return ShellExport{File: os.Getenv(composeusecase.ExportFileVar), Dialect: dialect}
}

func (a *App) DetectShell() string {
	return composeusecase.DetectShell(os.Getenv(shellEnvVar))
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

func (a *App) Presenter() *presenter.Presenter {
	return presenter.New(a.Stdout, a.Stderr)
}

func isTerminal(stream any) bool {
	file, ok := stream.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}
