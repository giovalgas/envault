package viewmodel

import (
	"fmt"
	"slices"
	"strings"

	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	MarkOn  = "[x]"
	MarkOff = "[ ]"

	selectedEnvsPrefix = "SELECTED_ENVS="
)

type Selection struct {
	names []string
}

func NewSelection(names []string) Selection {
	return Selection{names: slices.Clone(names)}
}

func (s Selection) Names() []string {
	return slices.Clone(s.names)
}

func (s Selection) Len() int {
	return len(s.names)
}

func (s Selection) Contains(name string) bool {
	return slices.Contains(s.names, name)
}

func (s Selection) Position(name string) int {
	return slices.Index(s.names, name) + 1
}

func (s Selection) Toggle(name string) Selection {
	if s.Contains(name) {
		return Selection{names: slices.DeleteFunc(slices.Clone(s.names), func(n string) bool { return n == name })}
	}
	return Selection{names: append(slices.Clone(s.names), name)}
}

func (s Selection) Keep(exists func(name string) bool) Selection {
	return Selection{names: slices.DeleteFunc(slices.Clone(s.names), func(name string) bool {
		return !exists(name)
	})}
}

func (s Selection) Rename(oldName, newName string) Selection {
	i := slices.Index(s.names, oldName)
	if i < 0 {
		return s
	}
	names := slices.Clone(s.names)
	names[i] = newName
	return Selection{names: names}
}

func (s Selection) Move(index, delta int) (Selection, bool) {
	next := index + delta
	if index < 0 || index >= len(s.names) || next < 0 || next >= len(s.names) {
		return s, false
	}
	names := slices.Clone(s.names)
	names[index], names[next] = names[next], names[index]
	return Selection{names: names}, true
}

func (s Selection) Numbered() []string {
	items := make([]string, len(s.names))
	for i, name := range s.names {
		items[i] = fmt.Sprintf("%d. %s", i+1, name)
	}
	return items
}

func (s Selection) MarkLabel(name string) (label string, marked bool) {
	width := max(len(MarkOff), len(fmt.Sprintf("[%d]", len(s.names))))
	pos := s.Position(name)
	if pos == 0 {
		return fmt.Sprintf("%-*s", width, MarkOff), false
	}
	return fmt.Sprintf("%-*s", width, fmt.Sprintf("[%d]", pos)), true
}

func (s Selection) SelectedEnvsLine() (text string, empty bool) {
	if len(s.names) == 0 {
		return selectedEnvsPrefix, true
	}
	return selectedEnvsPrefix + strings.Join(s.names, ","), false
}

func (s Selection) Pick(envs []vaultusecase.EnvView) []vaultusecase.EnvView {
	out := make([]vaultusecase.EnvView, 0, len(s.names))
	for _, name := range s.names {
		for _, env := range envs {
			if env.Name == name {
				out = append(out, env.Clone())
				break
			}
		}
	}
	return out
}
