package gitignore

import (
	"errors"
	"os/exec"
	"path/filepath"
)

type Status int

const (
	Unknown Status = iota
	Ignored
	NotIgnored
)

const checkIgnoreNotMatched = 1

func (s Status) Known() bool {
	return s == Ignored || s == NotIgnored
}

func (s Status) String() string {
	switch s {
	case Ignored:
		return "ignored"
	case NotIgnored:
		return "not-ignored"
	default:
		return "unknown"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	switch s {
	case Ignored:
		return []byte("true"), nil
	case NotIgnored:
		return []byte("false"), nil
	default:
		return []byte("null"), nil
	}
}

func IsIgnored(path string) Status {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Unknown
	}
	cmd := exec.Command("git", "-C", filepath.Dir(abs), "check-ignore", "-q", "--", abs)
	err = cmd.Run()
	if err == nil {
		return Ignored
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == checkIgnoreNotMatched {
		return NotIgnored
	}
	return Unknown
}
