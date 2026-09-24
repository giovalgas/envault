package presenter

import (
	"errors"
	"fmt"
	"strings"
	"time"

	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

var errSetMissingEquals = errors.New("informe KEY=VALUE ou uma única KEY para ler do stdin")

type ListEnvelope struct {
	SchemaVersion int            `json:"schema_version"`
	Envs          []ListEnvEntry `json:"envs"`
}

type ListEnvEntry struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Keys        []string  `json:"keys"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ShowEnvelope struct {
	SchemaVersion int       `json:"schema_version"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Tags          []string  `json:"tags"`
	Keys          []string  `json:"keys"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (p *Presenter) VaultInitialized(result vaultusecase.InitVaultResult) error {
	if result.Created {
		return p.Infof("cofre criado em %s", result.Location)
	}
	return p.Infof("cofre já inicializado em %s", result.Location)
}

func (p *Presenter) EnvList(envs []vaultusecase.EnvView, asJSON bool) error {
	if asJSON {
		return writeJSON(p.stdout, listBuildEnvelope(envs))
	}
	return p.listPrintHuman(envs)
}

func listBuildEnvelope(envs []vaultusecase.EnvView) ListEnvelope {
	entries := make([]ListEnvEntry, len(envs))
	for i, e := range envs {
		entries[i] = ListEnvEntry{
			Name:        e.Name,
			Description: e.Description,
			Tags:        nonNil(e.Tags),
			Keys:        nonNil(e.Keys()),
			UpdatedAt:   e.UpdatedAt,
		}
	}
	return ListEnvelope{SchemaVersion: SchemaVersion, Envs: entries}
}

func (p *Presenter) listPrintHuman(envs []vaultusecase.EnvView) error {
	if len(envs) == 0 {
		return p.Infof("nenhuma env encontrada")
	}
	var b strings.Builder
	for _, e := range envs {
		fmt.Fprintf(&b, "%s\n", e.Name)
		if e.Description != "" {
			fmt.Fprintf(&b, "  descrição: %s\n", e.Description)
		}
		if len(e.Tags) > 0 {
			fmt.Fprintf(&b, "  tags: %s\n", joined(e.Tags))
		}
		fmt.Fprintf(&b, "  chaves: %s\n", joined(e.Keys()))
		fmt.Fprintf(&b, "  atualizado em: %s\n", e.UpdatedAt.Format(time.RFC3339))
	}
	return p.writeText(b.String())
}

func (p *Presenter) Env(env vaultusecase.EnvView, asJSON bool) error {
	if asJSON {
		return writeJSON(p.stdout, showBuildEnvelope(env))
	}
	return p.showPrintHuman(env)
}

func showBuildEnvelope(env vaultusecase.EnvView) ShowEnvelope {
	return ShowEnvelope{
		SchemaVersion: SchemaVersion,
		Name:          env.Name,
		Description:   env.Description,
		Tags:          nonNil(env.Tags),
		Keys:          nonNil(env.Keys()),
		CreatedAt:     env.CreatedAt,
		UpdatedAt:     env.UpdatedAt,
	}
}

func (p *Presenter) showPrintHuman(env vaultusecase.EnvView) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", env.Name)
	if env.Description != "" {
		fmt.Fprintf(&b, "  descrição: %s\n", env.Description)
	}
	if len(env.Tags) > 0 {
		fmt.Fprintf(&b, "  tags: %s\n", joined(env.Tags))
	}
	fmt.Fprintf(&b, "  chaves: %s\n", joined(env.Keys()))
	fmt.Fprintf(&b, "  criada em: %s\n", env.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "  atualizada em: %s\n", env.UpdatedAt.Format(time.RFC3339))
	return p.writeText(b.String())
}

func (p *Presenter) Value(value string) error {
	_, err := fmt.Fprintln(p.stdout, value)
	return err
}

func (p *Presenter) ValuesSet(name string, count int) error {
	return p.Infof("definida(s) %d chave(s) em %q", count, name)
}

func (p *Presenter) KeysUnset(name string, removed []string) error {
	if len(removed) == 0 {
		return p.Infof("nenhuma chave removida em %q", name)
	}
	return p.Infof("removida(s) de %q: %s", name, joined(removed))
}

func (p *Presenter) EnvCreated(result vaultusecase.CreateEnvResult) error {
	if !result.Created {
		return withExitCode(ExitCanceled, ErrCanceled)
	}
	return p.Infof("env %q criada com %d chave(s)", result.Env.Name, len(result.Env.Vars))
}

func (p *Presenter) EnvEdited(name string, result vaultusecase.EditResultView) error {
	if !result.Changed {
		return p.Infof("nada mudou em %q", name)
	}
	if err := p.Infof("env %q atualizada:", name); err != nil {
		return err
	}
	for _, line := range result.Diff.Lines() {
		if err := p.Infof("  %s", line); err != nil {
			return err
		}
	}
	return nil
}

func (p *Presenter) EnvImported(name, path string, result vaultusecase.ImportEnvResult) error {
	verb := "criada"
	if result.Replaced {
		verb = "substituída"
	}
	return p.Infof("env %q %s com %d chave(s) de %s", name, verb, len(result.Env.Vars), path)
}

func (p *Presenter) EnvRenamed(from, to string) error {
	return p.Infof("%q renomeada para %q", from, to)
}

func (p *Presenter) EnvCopied(from, to string) error {
	return p.Infof("%q duplicada para %q", from, to)
}

func (p *Presenter) EnvDeleted(name string) error {
	return p.Infof("env %q removida", name)
}

func (p *Presenter) ConfirmDelete(name string) error {
	return p.Infof("digite %q para confirmar a exclusão:", name)
}

func (p *Presenter) KeyMigrated(result vaultusecase.MigrateKeyResult) error {
	if !result.Migrated {
		return p.Infof("a chave já está no keychain (serviço %s, conta %s); nada a migrar", result.Service, result.Account)
	}
	return p.Infof("chave migrada para o keychain (serviço %s, conta %s); arquivo %s removido", result.Service, result.Account, result.File)
}

func EditorError(err error) error {
	if errors.Is(err, vaultusecase.ErrEditCanceled) {
		return withExitCode(ExitCanceled, err)
	}
	return err
}

func DeleteNeedsConfirmation() error {
	return UsageError(errors.New("confirmação necessária: use --yes ou rode num terminal"))
}

func ReadConfirmationError(err error) error {
	return fmt.Errorf("ler confirmação: %w", err)
}

func ReadStdinError(err error) error {
	return fmt.Errorf("ler valor do stdin: %w", err)
}

func MissingEquals(arg string) error {
	return UsageError(fmt.Errorf("%w: %q", errSetMissingEquals, arg))
}
