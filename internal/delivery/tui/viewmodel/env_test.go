package viewmodel

import (
	"strings"
	"testing"
	"time"

	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

func sampleEnv() vaultusecase.EnvView {
	return vaultusecase.EnvView{
		Name:        "postgres-local",
		Description: "linha 1\nlinha 2",
		Tags:        []string{"db", "local"},
		Vars: []vaultusecase.VarView{
			{Key: "DATABASE_URL", Value: "valor-database"},
			{Key: "PGPASSWORD", Value: "valor\tcom\nquebra"},
		},
	}
}

func TestMaskedRowsNeverCarryValues(t *testing.T) {
	for _, row := range MaskedRows(sampleEnv()) {
		if row.Value != MaskedValue || row.Revealed {
			t.Fatalf("linha %s não mascarada: %+v", row.Key, row)
		}
	}
}

func TestDetailRowsRevealOnlyFocused(t *testing.T) {
	tests := []struct {
		name   string
		focus  int
		reveal bool
		want   []string
	}{
		{name: "tudo mascarado", focus: 0, reveal: false, want: []string{MaskedValue, MaskedValue}},
		{name: "revela a primeira", focus: 0, reveal: true, want: []string{"valor-database", MaskedValue}},
		{name: "revela a segunda numa linha só", focus: 1, reveal: true, want: []string{MaskedValue, `valor\tcom\nquebra`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := DetailRows(sampleEnv(), tt.focus, tt.reveal)
			for i, row := range rows {
				if row.Value != tt.want[i] || row.Revealed != (row.Value != MaskedValue) {
					t.Fatalf("linha %d = %+v, esperado %q", i, row, tt.want[i])
				}
			}
		})
	}
}

func TestHeaderAndMeta(t *testing.T) {
	h := Header(sampleEnv())
	if h.Description != `linha 1\nlinha 2` || h.Tags != "db, local" || h.Name != "postgres-local" {
		t.Fatalf("header = %+v", h)
	}
	empty := Header(vaultusecase.EnvView{Name: "vazia", Description: "  "})
	if empty.Description != "-" || empty.Tags != "-" {
		t.Fatalf("header vazio = %+v", empty)
	}
	if got := ListMeta(vaultusecase.EnvView{}); got != "0 chaves  -" {
		t.Fatalf("meta sem data = %q", got)
	}
	when := time.Date(2026, 9, 23, 12, 30, 0, 0, time.Local)
	if got := ListMeta(vaultusecase.EnvView{Vars: []vaultusecase.VarView{{Key: "K"}}, UpdatedAt: when}); got != "1 chave  2026-09-23 12:30" {
		t.Fatalf("meta = %q", got)
	}
}

func TestKeyCount(t *testing.T) {
	for n, want := range map[int]string{0: "0 chaves", 1: "1 chave", 2: "2 chaves"} {
		if got := KeyCount(n); got != want {
			t.Errorf("KeyCount(%d) = %q", n, got)
		}
	}
}

func TestSingleLine(t *testing.T) {
	if got := SingleLine("a\r\nb\tc"); strings.ContainsAny(got, "\r\n\t") {
		t.Fatalf("SingleLine = %q", got)
	}
}
