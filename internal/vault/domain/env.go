package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/giovalgas/envault/internal/shared/dotenv"
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type Var struct {
	Key   string
	Value string
}

type Env struct {
	Name        string
	Description string
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Vars        []Var
}

func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	return nil
}

func ValidateKey(key string) error {
	return dotenv.ValidateKey(key)
}

func ValidateTag(tag string) error {
	return dotenv.ValidateTag(tag)
}

func ValidateDescription(description string) error {
	if strings.ContainsAny(description, "\r\n") {
		return fmt.Errorf("%w: quebra de linha não é permitida", ErrInvalidDescription)
	}
	return nil
}

func FromDocument(doc dotenv.Document) Env {
	env := Env{Description: doc.Description, Tags: slices.Clone(doc.Tags)}
	if doc.Vars != nil {
		env.Vars = make([]Var, len(doc.Vars))
		for i, v := range doc.Vars {
			env.Vars[i] = Var(v)
		}
	}
	return env
}

func (e Env) Document() dotenv.Document {
	doc := dotenv.Document{Description: e.Description, Tags: slices.Clone(e.Tags)}
	if e.Vars != nil {
		doc.Vars = make([]dotenv.Var, len(e.Vars))
		for i, v := range e.Vars {
			doc.Vars[i] = dotenv.Var(v)
		}
	}
	return doc
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
