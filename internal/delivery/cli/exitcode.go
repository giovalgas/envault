package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/shared/dotenv"
	vault "github.com/giovalgas/envault/internal/vault/domain"
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
	jsonFlag             = "json"
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
	vault.ErrInvalidName,
	vault.ErrInvalidKey,
	vault.ErrInvalidTag,
	vault.ErrInvalidDescription,
	vault.ErrEnvExists,
	dotenv.ErrSyntax,
	vault.ErrMalformedKey,
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

func usageError(err error) error {
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
	case errors.Is(err, vault.ErrEnvNotFound):
		return ExitEnvNotFound
	case errors.Is(err, vault.ErrNotInitialized):
		return ExitNotInitialized
	case errors.Is(err, ErrTargetExists):
		return ExitTargetExists
	case errors.Is(err, vault.ErrDecrypt):
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

func alwaysJSON(cmd *cobra.Command) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[jsonOutputAnnotation] = jsonOutputAlways
	return cmd
}

func wantsJSON(cmd *cobra.Command, args []string) bool {
	if cmd != nil {
		if cmd.Annotations[jsonOutputAnnotation] == jsonOutputAlways {
			return true
		}
		if f := cmd.Flags().Lookup(jsonFlag); f != nil && f.Changed && f.Value.String() == "true" {
			return true
		}
	}
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--"+jsonFlag || arg == "--"+jsonFlag+"=true" {
			return true
		}
	}
	return false
}

func reportError(app *App, cmd *cobra.Command, args []string, err error) int {
	code := ExitCode(err)
	var coded *exitError
	if errors.As(err, &coded) && coded.err == nil {
		return code
	}
	if wantsJSON(cmd, args) && writeErrorJSON(app.Stdout, code, err) == nil {
		return code
	}
	fmt.Fprintf(app.Stderr, "envault: %s\n", err)
	if code == ExitUsage && cmd != nil {
		fmt.Fprintf(app.Stderr, "Veja %q.\n", cmd.CommandPath()+" --help")
	}
	return code
}
