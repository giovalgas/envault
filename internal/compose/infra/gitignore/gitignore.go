package gitignore

import (
	"errors"
	"os/exec"
	"path/filepath"

	"github.com/giovalgas/envault/internal/compose/domain"
)

const checkIgnoreNotMatched = 1

type Checker struct{}

func New() Checker {
	return Checker{}
}

func (Checker) IsIgnored(path string) domain.GitignoreStatus {
	return IsIgnored(path)
}

func IsIgnored(path string) domain.GitignoreStatus {
	abs, err := filepath.Abs(path)
	if err != nil {
		return domain.GitignoreUnknown
	}
	cmd := exec.Command("git", "-C", filepath.Dir(abs), "check-ignore", "-q", "--", abs)
	err = cmd.Run()
	if err == nil {
		return domain.GitignoreIgnored
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == checkIgnoreNotMatched {
		return domain.GitignoreNotIgnored
	}
	return domain.GitignoreUnknown
}
