package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/giovalgas/envault/internal/shared/dotenv"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func TestExitCodeMapping(t *testing.T) {
	_, parseErr := dotenv.Parse([]byte("1KEY=x"))
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, ExitOK},
		{"genérico", errors.New("boom"), ExitError},
		{"uso", usageError(errors.New("flag")), ExitUsage},
		{"env não encontrada", fmt.Errorf("show: %w", vault.ErrEnvNotFound), ExitEnvNotFound},
		{"não inicializado", fmt.Errorf("list: %w", vault.ErrNotInitialized), ExitNotInitialized},
		{"destino existe", fmt.Errorf("load: %w", ErrTargetExists), ExitTargetExists},
		{"decifrar", fmt.Errorf("abrir: %w", vault.ErrDecrypt), ExitDecrypt},
		{"sem chave com cofre", fmt.Errorf("%w: %w", vault.ErrDecrypt, vault.ErrNoKey), ExitDecrypt},
		{"nome inválido", vault.ErrInvalidName, ExitValidation},
		{"chave inválida", vault.ErrInvalidKey, ExitValidation},
		{"tag inválida", vault.ErrInvalidTag, ExitValidation},
		{"descrição inválida", vault.ErrInvalidDescription, ExitValidation},
		{"env já existe", vault.ErrEnvExists, ExitValidation},
		{"parse", parseErr, ExitValidation},
		{"ENVAULT_KEY inválida", vault.ErrMalformedKey, ExitValidation},
		{"validação genérica", ErrValidation, ExitValidation},
		{"cancelado", ErrCanceled, ExitCanceled},
		{"contexto cancelado", context.Canceled, ExitCanceled},
		{"código explícito", withExitCode(42, errors.New("filho")), 42},
		{"código explícito embrulhado", fmt.Errorf("exec: %w", withExitCode(9, nil)), 9},
		{"schema futuro", vault.ErrUnsupportedSchema, ExitError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCode(tc.err); got != tc.want {
				t.Fatalf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestWriteErrorJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := writeErrorJSON(&buf, ExitEnvNotFound, fmt.Errorf("%w: %q", vault.ErrEnvNotFound, "nao-existe")); err != nil {
		t.Fatalf("writeErrorJSON: %v", err)
	}
	want := `{"schema_version":1,"error":{"code":3,"message":"env não encontrada: \"nao-existe\""}}` + "\n"
	if buf.String() != want {
		t.Fatalf("got  %s\nwant %s", buf.String(), want)
	}
}

func TestWriteJSONDoesNotEscapeHTML(t *testing.T) {
	var buf bytes.Buffer
	if err := writeJSON(&buf, map[string]string{"k": "<a&b>"}); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	if buf.String() != `{"k":"<a&b>"}`+"\n" {
		t.Fatalf("got %s", buf.String())
	}
}

func TestExitErrorMessages(t *testing.T) {
	if got := withExitCode(3, nil).Error(); got != "código de saída 3" {
		t.Fatalf("Error() = %q", got)
	}
	inner := errors.New("inner")
	wrapped := withExitCode(3, inner)
	if wrapped.Error() != "inner" || !errors.Is(wrapped, inner) {
		t.Fatalf("wrapped = %v", wrapped)
	}
}

func TestWantsJSON(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"list", "--json"}, true},
		{[]string{"list", "--json=true"}, true},
		{[]string{"list"}, false},
		{[]string{"exec", "-e", "a", "--", "tool", "--json"}, false},
	}
	for _, tc := range cases {
		if got := wantsJSON(nil, tc.args); got != tc.want {
			t.Errorf("wantsJSON(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestAlwaysJSONPreservesAnnotations(t *testing.T) {
	cmd := failingCmd(nil)
	cmd.Annotations = map[string]string{"outra": "x"}
	alwaysJSON(cmd)
	if cmd.Annotations["outra"] != "x" || cmd.Annotations[jsonOutputAnnotation] != jsonOutputAlways {
		t.Fatalf("annotations = %v", cmd.Annotations)
	}
}
