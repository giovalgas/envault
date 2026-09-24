package theme

import (
	"strings"
	"testing"
)

const maxFooterBindings = 6

func TestScrollWindow(t *testing.T) {
	tests := []struct {
		cursor, total, size int
		start, end          int
	}{
		{cursor: 0, total: 3, size: 10, start: 0, end: 3},
		{cursor: 0, total: 20, size: 5, start: 0, end: 5},
		{cursor: 10, total: 20, size: 5, start: 8, end: 13},
		{cursor: 19, total: 20, size: 5, start: 15, end: 20},
		{cursor: 0, total: 0, size: 5, start: 0, end: 0},
	}
	for _, tt := range tests {
		start, end := ScrollWindow(tt.cursor, tt.total, tt.size)
		if start != tt.start || end != tt.end {
			t.Errorf("ScrollWindow(%d, %d, %d) = %d, %d; esperado %d, %d", tt.cursor, tt.total, tt.size, start, end, tt.start, tt.end)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in    string
		limit int
		want  string
	}{
		{in: "curto", limit: 10, want: "curto"},
		{in: "comprido demais", limit: 8, want: "comprid" + Ellipsis},
		{in: "qualquer", limit: 0, want: ""},
	}
	for _, tt := range tests {
		if got := Truncate(tt.in, tt.limit); got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, esperado %q", tt.in, tt.limit, got, tt.want)
		}
	}
}

func TestPadRight(t *testing.T) {
	if got := PadRight("ab", 4); got != "ab  " {
		t.Fatalf("PadRight = %q", got)
	}
	if got := PadRight("abcdef", 4); got != "abcdef" {
		t.Fatalf("PadRight sem espaço = %q", got)
	}
}

func TestFooterModesStayShort(t *testing.T) {
	k := DefaultKeyMap()
	for name, h := range map[string]HelpBindings{
		"lista":    k.ListHelp(),
		"filtro":   k.FilterHelp(),
		"detalhe":  k.DetailHelp(),
		"montagem": k.ComposeHelp(),
		"modal":    k.ModalHelp(),
		"ajuda":    k.HelpScreenHelp(),
	} {
		if n := len(h.ShortHelp()); n > maxFooterBindings {
			t.Errorf("%s: %d atalhos no rodapé", name, n)
		}
		if len(h.FullHelp()) != len(k.AllGroups()) {
			t.Errorf("%s: ajuda completa sem todos os grupos", name)
		}
	}
}

func TestFullHelpShowsEveryBinding(t *testing.T) {
	k := DefaultKeyMap()
	view := FullHelp(DefaultStyles(), k, 120, 32)
	if !strings.Contains(view, "Atalhos") {
		t.Fatalf("ajuda sem título:\n%s", view)
	}
	for _, group := range k.AllGroups() {
		for _, b := range group {
			h := b.Help()
			if !strings.Contains(view, h.Key) || !strings.Contains(view, h.Desc) {
				t.Errorf("ajuda não mostra %q %q", h.Key, h.Desc)
			}
		}
	}
}
