package infra

import (
	"context"
	"fmt"
	"os"

	"github.com/giovalgas/envault/internal/skill/domain"
)

type HomeDir struct{}

func NewHomeDir() *HomeDir {
	return &HomeDir{}
}

func (h *HomeDir) Dir(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w: %w", domain.ErrNoHomeDir, err)
	}
	return domain.DefaultDir(home)
}
