package dotenv

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	ErrInvalidKey = errors.New("chave inválida")
	ErrInvalidTag = errors.New("tag inválida")
)

var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Var struct {
	Key   string
	Value string
}

type Document struct {
	Description string
	Tags        []string
	Vars        []Var
}

func ValidateKey(key string) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	return nil
}

func ValidateTag(tag string) error {
	switch {
	case tag == "":
		return fmt.Errorf("%w: tag vazia", ErrInvalidTag)
	case strings.ContainsFunc(tag, unicode.IsSpace):
		return fmt.Errorf("%w: %q contém espaço", ErrInvalidTag, tag)
	case strings.Contains(tag, ","):
		return fmt.Errorf("%w: %q contém vírgula", ErrInvalidTag, tag)
	case strings.ToLower(tag) != tag:
		return fmt.Errorf("%w: %q precisa ser minúscula", ErrInvalidTag, tag)
	}
	return nil
}
