package vault

import (
	"context"
	"fmt"
)

func (v *Vault) List(ctx context.Context) ([]Env, error) {
	snapshot, err := v.Load(ctx)
	if err != nil {
		return nil, err
	}
	return snapshot.Sorted(), nil
}

func (v *Vault) Get(ctx context.Context, name string) (Env, error) {
	snapshot, err := v.Load(ctx)
	if err != nil {
		return Env{}, err
	}
	env, ok := snapshot.Envs[name]
	if !ok {
		return Env{}, notFound(name)
	}
	return env.Clone(), nil
}

func (v *Vault) Create(ctx context.Context, env Env) (Env, error) {
	return v.store(ctx, env, func(exists bool) error {
		if exists {
			return fmt.Errorf("%w: %q", ErrEnvExists, env.Name)
		}
		return nil
	})
}

func (v *Vault) Replace(ctx context.Context, env Env) (Env, error) {
	return v.store(ctx, env, func(exists bool) error {
		if !exists {
			return notFound(env.Name)
		}
		return nil
	})
}

func (v *Vault) Put(ctx context.Context, env Env) (Env, error) {
	return v.store(ctx, env, func(bool) error { return nil })
}

func (v *Vault) Modify(ctx context.Context, name string, change func(*Env) error) (Env, error) {
	var result Env
	err := v.Update(ctx, func(s *Snapshot) error {
		current, ok := s.Envs[name]
		if !ok {
			return notFound(name)
		}
		edited := current.Clone()
		if err := change(&edited); err != nil {
			return err
		}
		edited.Name = name
		edited.CreatedAt = current.CreatedAt
		edited.UpdatedAt = v.timestamp()
		if err := edited.ValidateContent(); err != nil {
			return err
		}
		s.Envs[name] = edited
		result = edited.Clone()
		return nil
	})
	return result, err
}

func (v *Vault) Rename(ctx context.Context, oldName, newName string) (Env, error) {
	if err := ValidateName(newName); err != nil {
		return Env{}, err
	}
	var result Env
	err := v.Update(ctx, func(s *Snapshot) error {
		env, ok := s.Envs[oldName]
		if !ok {
			return notFound(oldName)
		}
		if oldName == newName {
			result = env.Clone()
			return nil
		}
		if _, taken := s.Envs[newName]; taken {
			return fmt.Errorf("%w: %q", ErrEnvExists, newName)
		}
		delete(s.Envs, oldName)
		env.Name = newName
		env.UpdatedAt = v.timestamp()
		s.Envs[newName] = env
		result = env.Clone()
		return nil
	})
	return result, err
}

func (v *Vault) Copy(ctx context.Context, src, dst string) (Env, error) {
	if err := ValidateName(dst); err != nil {
		return Env{}, err
	}
	var result Env
	err := v.Update(ctx, func(s *Snapshot) error {
		env, ok := s.Envs[src]
		if !ok {
			return notFound(src)
		}
		if _, taken := s.Envs[dst]; taken {
			return fmt.Errorf("%w: %q", ErrEnvExists, dst)
		}
		copied := env.Clone()
		now := v.timestamp()
		copied.Name = dst
		copied.CreatedAt = now
		copied.UpdatedAt = now
		s.Envs[dst] = copied
		result = copied.Clone()
		return nil
	})
	return result, err
}

func (v *Vault) Delete(ctx context.Context, name string) error {
	return v.Update(ctx, func(s *Snapshot) error {
		if _, ok := s.Envs[name]; !ok {
			return notFound(name)
		}
		delete(s.Envs, name)
		return nil
	})
}

func (v *Vault) store(ctx context.Context, env Env, check func(exists bool) error) (Env, error) {
	if err := env.Validate(); err != nil {
		return Env{}, err
	}
	var result Env
	err := v.Update(ctx, func(s *Snapshot) error {
		current, exists := s.Envs[env.Name]
		if err := check(exists); err != nil {
			return err
		}
		stored := env.Clone()
		now := v.timestamp()
		stored.CreatedAt = now
		if exists {
			stored.CreatedAt = current.CreatedAt
		}
		stored.UpdatedAt = now
		s.Envs[env.Name] = stored
		result = stored.Clone()
		return nil
	})
	return result, err
}

func notFound(name string) error {
	return fmt.Errorf("%w: %q", ErrEnvNotFound, name)
}
