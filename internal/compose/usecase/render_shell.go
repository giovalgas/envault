package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
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

func DetectShell(shellPath string) string {
	detected := strings.TrimSuffix(filepath.Base(shellPath), ".exe")
	if slices.Contains(ShellDialects(), detected) {
		return detected
	}
	return ShellBash
}

type RenderShellInput struct {
	Envs         []string
	Dialect      string
	Template     TemplateView
	OnlyTemplate bool
}

type RenderShellResult struct {
	Plan   PlanView
	Script string
}

type RenderShell struct {
	envs EnvReader
}

func NewRenderShell(envs EnvReader) *RenderShell {
	return &RenderShell{envs: envs}
}

func (uc *RenderShell) Execute(ctx context.Context, in RenderShellInput) (RenderShellResult, error) {
	plan, err := combine(ctx, uc.envs, in.Envs, in.Template, in.OnlyTemplate)
	if err != nil {
		return RenderShellResult{}, err
	}
	return RenderShellResult{Plan: planView(plan), Script: renderShell(in.Dialect, plan.Pairs())}, nil
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
