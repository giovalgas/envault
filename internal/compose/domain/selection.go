package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

var ErrInvalidSelection = errors.New("seleção inválida")

type Selection struct {
	Envs      []string
	UpdatedAt time.Time
}

func NewSelection(names []string, updatedAt time.Time) (Selection, error) {
	envs := make([]string, 0, len(names))
	for _, name := range names {
		if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) {
			return Selection{}, fmt.Errorf("%w: nome de env %q", ErrInvalidSelection, name)
		}
		if slices.Contains(envs, name) {
			return Selection{}, fmt.Errorf("%w: env %q repetida", ErrInvalidSelection, name)
		}
		envs = append(envs, name)
	}
	return Selection{Envs: envs, UpdatedAt: updatedAt}, nil
}

func (s Selection) Saved() bool {
	return !s.UpdatedAt.IsZero()
}

func (s Selection) Split(existing []string) (kept Selection, missing []string) {
	known := make(map[string]bool, len(existing))
	for _, name := range existing {
		known[name] = true
	}
	kept = Selection{Envs: make([]string, 0, len(s.Envs)), UpdatedAt: s.UpdatedAt}
	missing = make([]string, 0)
	for _, name := range s.Envs {
		if known[name] {
			kept.Envs = append(kept.Envs, name)
			continue
		}
		missing = append(missing, name)
	}
	return kept, missing
}
