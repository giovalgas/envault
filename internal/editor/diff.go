package editor

import (
	"slices"
	"strings"

	"github.com/giovalgas/envault/internal/vault"
)

type Diff struct {
	Added     []string
	Removed   []string
	Changed   []string
	Metadata  bool
	Reordered bool
}

func Compare(before, after vault.Env) Diff {
	var d Diff
	for _, v := range after.Vars {
		old, ok := before.Lookup(v.Key)
		switch {
		case !ok:
			d.Added = append(d.Added, v.Key)
		case old != v.Value:
			d.Changed = append(d.Changed, v.Key)
		}
	}
	for _, v := range before.Vars {
		if _, ok := after.Lookup(v.Key); !ok {
			d.Removed = append(d.Removed, v.Key)
		}
	}
	d.Metadata = before.Description != after.Description || !slices.Equal(before.Tags, after.Tags)
	d.Reordered = len(d.Added) == 0 && len(d.Removed) == 0 && !slices.Equal(before.Keys(), after.Keys())
	return d
}

func (d Diff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0 && !d.Metadata && !d.Reordered
}

func (d Diff) Lines() []string {
	lines := make([]string, 0, len(d.Added)+len(d.Removed)+len(d.Changed)+2)
	for _, key := range d.Added {
		lines = append(lines, "+ "+key)
	}
	for _, key := range d.Removed {
		lines = append(lines, "- "+key)
	}
	for _, key := range d.Changed {
		lines = append(lines, "~ "+key)
	}
	if d.Metadata {
		lines = append(lines, "~ descrição ou tags")
	}
	if d.Reordered {
		lines = append(lines, "~ ordem das chaves")
	}
	return lines
}

func (d Diff) String() string {
	lines := d.Lines()
	if len(lines) == 0 {
		return "nenhuma mudança"
	}
	return strings.Join(lines, "\n")
}
