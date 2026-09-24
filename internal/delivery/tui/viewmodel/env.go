package viewmodel

import (
	"fmt"
	"strings"

	"github.com/giovalgas/envault/internal/delivery/tui/theme"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	MaskedValue    = "••••••••"
	DefaultEnvFile = ".env"
	timeLayout     = "2006-01-02 15:04"
)

type EnvHeader struct {
	Name        string
	Description string
	Tags        string
}

type VarRow struct {
	Key      string
	Value    string
	Revealed bool
}

func Header(env vaultusecase.EnvView) EnvHeader {
	return EnvHeader{
		Name:        env.Name,
		Description: OrDash(env.Description),
		Tags:        OrDash(strings.Join(env.Tags, ", ")),
	}
}

func MaskedRows(env vaultusecase.EnvView) []VarRow {
	return DetailRows(env, -1, false)
}

func DetailRows(env vaultusecase.EnvView, focus int, reveal bool) []VarRow {
	rows := make([]VarRow, len(env.Vars))
	for i, v := range env.Vars {
		rows[i] = VarRow{Key: v.Key, Value: MaskedValue}
		if reveal && i == focus {
			rows[i] = VarRow{Key: v.Key, Value: SingleLine(v.Value), Revealed: true}
		}
	}
	return rows
}

func KeyColumn(row VarRow, width int) string {
	keyWidth := max(width-len(row.Value)-len(theme.KeyValueSeparator), 1)
	return theme.Truncate(row.Key, keyWidth)
}

func ListMeta(env vaultusecase.EnvView) string {
	return fmt.Sprintf("%s  %s", KeyCount(len(env.Vars)), UpdatedAt(env))
}

func KeyCount(n int) string {
	if n == 1 {
		return "1 chave"
	}
	return fmt.Sprintf("%d chaves", n)
}

func UpdatedAt(env vaultusecase.EnvView) string {
	if env.UpdatedAt.IsZero() {
		return "-"
	}
	return env.UpdatedAt.Local().Format(timeLayout)
}

func OrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return SingleLine(s)
}

func SingleLine(s string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`, "\t", `\t`).Replace(s)
}
