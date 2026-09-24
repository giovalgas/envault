package editor

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mattn/go-shellwords"
)

const (
	EnvVisual = "VISUAL"
	EnvEditor = "EDITOR"

	goosWindows = "windows"
	waitFlag    = "--wait"
)

var ErrNoEditor = errors.New("editor inválido")

var (
	waitingEditors     = []string{"code", "codium", "subl", "zed"}
	waitFlags          = []string{waitFlag, "-w"}
	windowsExecutables = []string{".exe", ".cmd", ".bat"}
)

type LookupFunc func(key string) (string, bool)

type Spec struct {
	Name     string
	Args     []string
	Warnings []string
}

func (s Spec) Argv(path string) []string {
	argv := append([]string{s.Name}, s.Args...)
	return append(argv, path)
}

func Resolve(lookup LookupFunc, goos string) (Spec, error) {
	source, raw := chooseEditor(lookup, goos)
	words, err := splitCommandLine(raw, goos)
	if err != nil {
		return Spec{}, fmt.Errorf("%w: $%s=%q: %w", ErrNoEditor, source, raw, err)
	}
	if len(words) == 0 || words[0] == "" {
		return Spec{}, fmt.Errorf("%w: $%s=%q não tem programa", ErrNoEditor, source, raw)
	}
	return adjust(Spec{Name: words[0], Args: words[1:]}), nil
}

func DefaultEditor(goos string) string {
	if goos == goosWindows {
		return "notepad"
	}
	return "vi"
}

func chooseEditor(lookup LookupFunc, goos string) (source, raw string) {
	for _, key := range []string{EnvVisual, EnvEditor} {
		if value, ok := lookup(key); ok && strings.TrimSpace(value) != "" {
			return key, strings.TrimSpace(value)
		}
	}
	return "padrão", DefaultEditor(goos)
}

func splitCommandLine(raw, goos string) ([]string, error) {
	if goos == goosWindows {
		raw = literalBackslashes(raw)
	}
	parser := shellwords.NewParser()
	parser.ParseEnv = false
	parser.ParseBacktick = false
	words, err := parser.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parser.Position >= 0 {
		return nil, errors.New("operadores de shell não são suportados")
	}
	return words, nil
}

func literalBackslashes(raw string) string {
	var b strings.Builder
	var singleQuoted, doubleQuoted bool
	for _, r := range raw {
		switch {
		case r == '\'' && !doubleQuoted:
			singleQuoted = !singleQuoted
		case r == '"' && !singleQuoted:
			doubleQuoted = !doubleQuoted
		case r == '\\' && !singleQuoted:
			b.WriteRune(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

func adjust(spec Spec) Spec {
	base := programName(spec.Name)
	switch {
	case slices.Contains(waitingEditors, base):
		if !containsAny(spec.Args, waitFlags...) {
			spec.Args = append(spec.Args, waitFlag)
			spec.Warnings = append(spec.Warnings, fmt.Sprintf(
				"aviso: %s sem %s; acrescentei %s para esperar o editor fechar", base, waitFlag, waitFlag))
		}
	case base == "vim" || base == "nvim":
		if !slices.Contains(spec.Args, "-n") {
			spec.Args = append(spec.Args, "-n")
		}
		if base == "vim" && !slices.Contains(spec.Args, "-i") {
			spec.Args = append(spec.Args, "-i", "NONE")
		}
	}
	return spec
}

func programName(name string) string {
	base := strings.ToLower(filepath.Base(strings.ReplaceAll(name, `\`, "/")))
	for _, ext := range windowsExecutables {
		if trimmed, ok := strings.CutSuffix(base, ext); ok {
			return trimmed
		}
	}
	return base
}

func containsAny(args []string, wanted ...string) bool {
	return slices.ContainsFunc(args, func(arg string) bool { return slices.Contains(wanted, arg) })
}
