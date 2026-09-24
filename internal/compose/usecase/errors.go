package usecase

import (
	"errors"
	"fmt"

	"github.com/giovalgas/envault/internal/compose/domain"
	"github.com/giovalgas/envault/internal/shared/dotenv"
)

var (
	ErrTargetExists      = errors.New("arquivo de destino já existe")
	ErrTargetIsDirectory = errors.New("destino é um diretório")
	ErrNoExportFile      = errors.New("arquivo de export do shell não definido")
	ErrUnsupportedShell  = errors.New("shell não suportado")
	ErrTemplateNotFound  = errors.New("template não existe")
	ErrTemplateRequired  = errors.New("só as chaves do template exigem um template")
	ErrNoCommand         = errors.New("nenhum comando para executar")
	ErrInvalidSelection  = domain.ErrInvalidSelection
	ErrSyntax            = dotenv.ErrSyntax
)

type DuplicateEnvError struct {
	Name string
}

func (e *DuplicateEnvError) Error() string {
	return fmt.Sprintf("env %q repetida", e.Name)
}

type EnvNotFoundError struct {
	Name string
}

func (e *EnvNotFoundError) Error() string {
	return fmt.Sprintf("env %q não encontrada", e.Name)
}

type TemplateReadError struct {
	Path string
	Err  error
}

func (e *TemplateReadError) Error() string {
	return fmt.Sprintf("ler template %s: %v", e.Path, e.Err)
}

func (e *TemplateReadError) Unwrap() error {
	return e.Err
}

type TemplateParseError struct {
	Path string
	Err  error
}

func (e *TemplateParseError) Error() string {
	return fmt.Sprintf("template %s: %v", e.Path, e.Err)
}

func (e *TemplateParseError) Unwrap() error {
	return e.Err
}
