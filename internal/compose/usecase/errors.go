package usecase

import (
	"errors"
	"fmt"
)

var (
	ErrTargetExists      = errors.New("arquivo de destino já existe")
	ErrTargetIsDirectory = errors.New("destino é um diretório")
)

type DuplicateEnvError struct {
	Name string
}

func (e *DuplicateEnvError) Error() string {
	return fmt.Sprintf("env %q repetida", e.Name)
}

type EnvNotFoundError struct {
	Name string
}

func (e *EnvNotFoundError) Error() string {
	return fmt.Sprintf("env %q não encontrada", e.Name)
}
