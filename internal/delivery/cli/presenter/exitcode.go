package presenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/spf13/cobra"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const SchemaVersion = 1

const (
	ExitOK             = 0
	ExitError          = 1
	ExitUsage          = 2
	ExitEnvNotFound    = 3
	ExitNotInitialized = 4
	ExitTargetExists   = 5
	ExitDecrypt        = 6
	ExitValidation     = 7
	ExitCanceled       = 130
)

const (
	JSONFlag             = "json"
	jsonOutputAnnotation = "envault/json-output"
	jsonOutputAlways     = "always"
)

var (
	ErrUsage        = errors.New("uso incorreto")
	ErrTargetExists = errors.New("arquivo de destino já existe")
	ErrValidation   = errors.New("erro de validação")
	ErrCanceled     = errors.New("cancelado pelo usuário")
)

var validationErrors = []error{
	ErrValidation,
	vaultusecase.ErrInvalidName,
	vaultusecase.ErrInvalidKey,
	vaultusecase.ErrInvalidTag,
	vaultusecase.ErrInvalidDescription,
	vaultusecase.ErrEnvExists,
	vaultusecase.ErrSyntax,
	composeusecase.ErrSyntax,
	vaultusecase.ErrMalformedKey,
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("código de saída %d", e.code)
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

func withExitCode(code int, err error) error {
	return &exitError{code: code, err: err}
}

func UsageError(err error) error {
	return fmt.Errorf("%w: %w", ErrUsage, err)
}

func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var coded *exitError
	if errors.As(err, &coded) {
		return coded.code
	}
	switch {
	case errors.Is(err, ErrCanceled), errors.Is(err, context.Canceled):
		return ExitCanceled
	case errors.Is(err, ErrUsage):
		return ExitUsage
	case errors.Is(err, vaultusecase.ErrEnvNotFound):
		return ExitEnvNotFound
	case errors.Is(err, vaultusecase.ErrNotInitialized):
		return ExitNotInitialized
	case errors.Is(err, ErrTargetExists):
		return ExitTargetExists
	case errors.Is(err, vaultusecase.ErrDecrypt):
		return ExitDecrypt
	case isValidation(err):
		return ExitValidation
	default:
		return ExitError
	}
}

func isValidation(err error) bool {
	return slices.ContainsFunc(validationErrors, func(target error) bool { return errors.Is(err, target) })
}

type errorEnvelope struct {
	SchemaVersion int       `json:"schema_version"`
	Error         errorBody `json:"error"`
}

type errorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func writeErrorJSON(w io.Writer, code int, err error) error {
	return writeJSON(w, errorEnvelope{
		SchemaVersion: SchemaVersion,
		Error:         errorBody{Code: code, Message: err.Error()},
	})
}

func AlwaysJSON(cmd *cobra.Command) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[jsonOutputAnnotation] = jsonOutputAlways
	return cmd
}

func WantsJSON(cmd *cobra.Command, args []string) bool {
	if cmd != nil {
		if cmd.Annotations[jsonOutputAnnotation] == jsonOutputAlways {
			return true
		}
		if f := cmd.Flags().Lookup(JSONFlag); f != nil && f.Changed && f.Value.String() == "true" {
			return true
		}
	}
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--"+JSONFlag || arg == "--"+JSONFlag+"=true" {
			return true
		}
	}
	return false
}

func (p *Presenter) Error(cmd *cobra.Command, args []string, err error) int {
	code := ExitCode(err)
	var coded *exitError
	if errors.As(err, &coded) && coded.err == nil {
		return code
	}
	if WantsJSON(cmd, args) && writeErrorJSON(p.stdout, code, err) == nil {
		return code
	}
	_, _ = io.WriteString(p.stderr, errorText(cmd, code, err))
	return code
}

func errorText(cmd *cobra.Command, code int, err error) string {
	text := fmt.Sprintf("envault: %s\n", err)
	if code == ExitUsage && cmd != nil {
		text += fmt.Sprintf("Veja %q.\n", cmd.CommandPath()+" --help")
	}
	return text
}
