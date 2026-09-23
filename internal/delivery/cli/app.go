package cli

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/shared/config"
	skillusecase "github.com/giovalgas/envault/internal/skill/usecase"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

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
}

type VaultDeps struct {
	Repository vault.EnvRepository
	Editor     vault.Editor
	Clock      vault.Clock
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
		CreateEnv: vaultusecase.NewCreateEnv(repo, deps.Editor, clock),
		EditEnv:   vaultusecase.NewEditEnv(repo, deps.Editor, clock),
		ImportEnv: vaultusecase.NewImportEnv(repo, clock),
		RenameEnv: vaultusecase.NewRenameEnv(repo, clock),
		CopyEnv:   vaultusecase.NewCopyEnv(repo, clock),
		DeleteEnv: vaultusecase.NewDeleteEnv(repo),
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

type ComposeUseCases struct {
	PlanLoad     *composeusecase.PlanLoad
	LoadEnvFile  *composeusecase.LoadEnvFile
	ExecWithEnvs *composeusecase.ExecWithEnvs
	RenderShell  *composeusecase.RenderShell
}

type ComposeDeps struct {
	Envs      composeusecase.EnvReader
	Files     composeusecase.TargetFiles
	Gitignore composeusecase.GitignoreChecker
}

func NewComposeUseCases(deps ComposeDeps) ComposeUseCases {
	return ComposeUseCases{
		PlanLoad:     composeusecase.NewPlanLoad(deps.Envs, deps.Files, deps.Gitignore),
		LoadEnvFile:  composeusecase.NewLoadEnvFile(deps.Envs, deps.Files, deps.Gitignore),
		ExecWithEnvs: composeusecase.NewExecWithEnvs(deps.Envs),
		RenderShell:  composeusecase.NewRenderShell(deps.Envs),
	}
}

type Wiring struct {
	Vault        func(cfg config.Config, streams Streams) (VaultUseCases, error)
	KeyMigration func(cfg config.Config) *vaultusecase.MigrateKey
	Compose      func(vault VaultOpener) ComposeUseCases
	Skill        func() *skillusecase.InstallSkill
}

type App struct {
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Version    string
	LoadConfig func() (config.Config, error)
	Wire       Wiring
	IsTerminal func(stream any) bool
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
	return a.Wire.Compose(a.Vault)
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
