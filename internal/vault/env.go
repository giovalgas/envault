package vault

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/giovalgas/envault/internal/crypto"
)

var (
	ErrNotInitialized     = errors.New("cofre não inicializado")
	ErrEnvNotFound        = errors.New("env não encontrada")
	ErrEnvExists          = errors.New("env já existe")
	ErrDecrypt            = crypto.ErrDecrypt
	ErrInvalidName        = errors.New("nome de env inválido")
	ErrInvalidKey         = errors.New("chave inválida")
	ErrInvalidTag         = errors.New("tag inválida")
	ErrInvalidDescription = errors.New("descrição inválida")
	ErrUnsupportedSchema  = errors.New("versão de schema não suportada")
)

var (
	namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	keyPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type Var struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Env struct {
	Name        string    `json:"-"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Vars        []Var     `json:"vars"`
}

func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	return nil
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

func ValidateDescription(description string) error {
	if strings.ContainsAny(description, "\r\n") {
		return fmt.Errorf("%w: quebra de linha não é permitida", ErrInvalidDescription)
	}
	return nil
}

func (e Env) Validate() error {
	if err := ValidateName(e.Name); err != nil {
		return err
	}
	return e.ValidateContent()
}

func (e Env) ValidateContent() error {
	if err := ValidateDescription(e.Description); err != nil {
		return err
	}
	for _, tag := range e.Tags {
		if err := ValidateTag(tag); err != nil {
			return err
		}
	}
	seen := make(map[string]struct{}, len(e.Vars))
	for _, v := range e.Vars {
		if err := ValidateKey(v.Key); err != nil {
			return err
		}
		if _, dup := seen[v.Key]; dup {
			return fmt.Errorf("%w: %q duplicada", ErrInvalidKey, v.Key)
		}
		seen[v.Key] = struct{}{}
	}
	return nil
}

func (e Env) Keys() []string {
	keys := make([]string, len(e.Vars))
	for i, v := range e.Vars {
		keys[i] = v.Key
	}
	return keys
}

func (e Env) Lookup(key string) (string, bool) {
	for _, v := range e.Vars {
		if v.Key == key {
			return v.Value, true
		}
	}
	return "", false
}

func (e *Env) Set(key, value string) {
	for i := range e.Vars {
		if e.Vars[i].Key == key {
			e.Vars[i].Value = value
			return
		}
	}
	e.Vars = append(e.Vars, Var{Key: key, Value: value})
}

func (e *Env) Unset(keys ...string) []string {
	var removed []string
	e.Vars = slices.DeleteFunc(e.Vars, func(v Var) bool {
		if slices.Contains(keys, v.Key) {
			removed = append(removed, v.Key)
			return true
		}
		return false
	})
	return removed
}

func (e Env) HasTag(tag string) bool {
	return slices.Contains(e.Tags, tag)
}

func (e Env) Clone() Env {
	out := e
	out.Tags = slices.Clone(e.Tags)
	out.Vars = slices.Clone(e.Vars)
	return out
}

func (e Env) SameContent(other Env) bool {
	return e.Description == other.Description &&
		slices.Equal(e.Tags, other.Tags) &&
		slices.Equal(e.Vars, other.Vars)
}

func (e Env) normalized() Env {
	out := e.Clone()
	if out.Tags == nil {
		out.Tags = []string{}
	}
	if out.Vars == nil {
		out.Vars = []Var{}
	}
	return out
}
