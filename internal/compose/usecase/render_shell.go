package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/giovalgas/envault/internal/compose/domain"
)

const (
	ShellBash = "bash"
	ShellZsh  = "zsh"
	ShellFish = "fish"
)

var (
	posixQuoter = strings.NewReplacer(`'`, `'\''`)
	fishQuoter  = strings.NewReplacer(`\`, `\\`, `'`, `\'`)
)

func ShellDialects() []string {
	return []string{ShellBash, ShellZsh, ShellFish}
}

type RenderShellInput struct {
	Envs    []string
	Dialect string
}

type RenderShell struct {
	envs EnvReader
}

func NewRenderShell(envs EnvReader) *RenderShell {
	return &RenderShell{envs: envs}
}

func (uc *RenderShell) Execute(ctx context.Context, in RenderShellInput) (string, error) {
	plan, err := combine(ctx, uc.envs, in.Envs, nil, domain.Options{})
	if err != nil {
		return "", err
	}
	return renderShell(in.Dialect, plan.Pairs()), nil
}

func renderShell(dialect string, vars []domain.Var) string {
	var b strings.Builder
	for _, v := range vars {
		if dialect == ShellFish {
			fmt.Fprintf(&b, "set -gx %s '%s'\n", v.Key, fishQuoter.Replace(v.Value))
			continue
		}
		fmt.Fprintf(&b, "export %s='%s'\n", v.Key, posixQuoter.Replace(v.Value))
	}
	return b.String()
}
