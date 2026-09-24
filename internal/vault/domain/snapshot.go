package domain

import (
	"fmt"
	"sort"
	"time"
)

type Snapshot struct {
	UpdatedAt time.Time
	Envs      map[string]Env
}

func NewSnapshot(updatedAt time.Time) *Snapshot {
	return &Snapshot{UpdatedAt: updatedAt, Envs: map[string]Env{}}
}

func (s *Snapshot) Names() []string {
	names := make([]string, 0, len(s.Envs))
	for name := range s.Envs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *Snapshot) Sorted() []Env {
	envs := make([]Env, 0, len(s.Envs))
	for _, name := range s.Names() {
		envs = append(envs, s.Envs[name].Clone())
	}
	return envs
}

func (s *Snapshot) Validate() error {
	for name, env := range s.Envs {
		env.Name = name
		if err := env.Validate(); err != nil {
			return err
		}
		s.Envs[name] = env
	}
	return nil
}

func (s *Snapshot) Has(name string) bool {
	_, ok := s.Envs[name]
	return ok
}

func (s *Snapshot) Get(name string) (Env, error) {
	env, ok := s.Envs[name]
	if !ok {
		return Env{}, notFound(name)
	}
	env.Name = name
	return env.Clone(), nil
}

func (s *Snapshot) Create(env Env, now time.Time) (Env, error) {
	return s.store(env, now, func(exists bool) error {
		if exists {
			return fmt.Errorf("%w: %q", ErrEnvExists, env.Name)
		}
		return nil
	})
}

func (s *Snapshot) Put(env Env, now time.Time) (Env, error) {
	return s.store(env, now, func(bool) error { return nil })
}

func (s *Snapshot) Modify(name string, now time.Time, change func(*Env) error) (Env, error) {
	current, ok := s.Envs[name]
	if !ok {
		return Env{}, notFound(name)
	}
	edited := current.Clone()
	if err := change(&edited); err != nil {
		return Env{}, err
	}
	edited.Name = name
	edited.CreatedAt = current.CreatedAt
	edited.UpdatedAt = now
	if err := edited.ValidateContent(); err != nil {
		return Env{}, err
	}
	s.Envs[name] = edited
	return edited.Clone(), nil
}

func (s *Snapshot) Rename(oldName, newName string, now time.Time) (Env, error) {
	if err := ValidateName(newName); err != nil {
		return Env{}, err
	}
	env, ok := s.Envs[oldName]
	if !ok {
		return Env{}, notFound(oldName)
	}
	if oldName == newName {
		return env.Clone(), nil
	}
	if _, taken := s.Envs[newName]; taken {
		return Env{}, fmt.Errorf("%w: %q", ErrEnvExists, newName)
	}
	delete(s.Envs, oldName)
	env.Name = newName
	env.UpdatedAt = now
	s.Envs[newName] = env
	return env.Clone(), nil
}

func (s *Snapshot) Copy(src, dst string, now time.Time) (Env, error) {
	if err := ValidateName(dst); err != nil {
		return Env{}, err
	}
	env, ok := s.Envs[src]
	if !ok {
		return Env{}, notFound(src)
	}
	if _, taken := s.Envs[dst]; taken {
		return Env{}, fmt.Errorf("%w: %q", ErrEnvExists, dst)
	}
	copied := env.Clone()
	copied.Name = dst
	copied.CreatedAt = now
	copied.UpdatedAt = now
	s.Envs[dst] = copied
	return copied.Clone(), nil
}

func (s *Snapshot) Delete(name string) error {
	if _, ok := s.Envs[name]; !ok {
		return notFound(name)
	}
	delete(s.Envs, name)
	return nil
}

func (s *Snapshot) store(env Env, now time.Time, check func(exists bool) error) (Env, error) {
	if err := env.Validate(); err != nil {
		return Env{}, err
	}
	current, exists := s.Envs[env.Name]
	if err := check(exists); err != nil {
		return Env{}, err
	}
	stored := env.Clone()
	stored.CreatedAt = now
	if exists {
		stored.CreatedAt = current.CreatedAt
	}
	stored.UpdatedAt = now
	s.Envs[env.Name] = stored
	return stored.Clone(), nil
}

func notFound(name string) error {
	return fmt.Errorf("%w: %q", ErrEnvNotFound, name)
}
