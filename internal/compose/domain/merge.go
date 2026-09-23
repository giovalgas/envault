package domain

type Var struct {
	Key   string
	Value string
}

type Env struct {
	Name string
	Vars []Var
}

type TemplateEntry struct {
	Key        string
	Default    string
	HasDefault bool
}

type Template struct {
	Entries []TemplateEntry
}

func (t Template) Keys() []string {
	keys := make([]string, len(t.Entries))
	for i, entry := range t.Entries {
		keys[i] = entry.Key
	}
	return keys
}

type Options struct {
	OnlyTemplate bool
}

type Source struct {
	Env   string
	Value string
}

type Resolved struct {
	Key     string
	Value   string
	From    string
	Shadows []Source
	Default bool
}

func (r Resolved) ShadowNames() []string {
	names := make([]string, len(r.Shadows))
	for i, shadow := range r.Shadows {
		names[i] = shadow.Env
	}
	return names
}

type Plan struct {
	Envs      []string
	Vars      []Resolved
	Conflicts []string
	Missing   []string
	Extra     []string
}

func (p Plan) Pairs() []Var {
	pairs := make([]Var, len(p.Vars))
	for i, resolved := range p.Vars {
		pairs[i] = Var{Key: resolved.Key, Value: resolved.Value}
	}
	return pairs
}

func (p Plan) Keys() []string {
	keys := make([]string, len(p.Vars))
	for i, resolved := range p.Vars {
		keys[i] = resolved.Key
	}
	return keys
}

type occurrences struct {
	order   []string
	sources map[string][]Source
}

func collect(envs []Env) occurrences {
	occ := occurrences{sources: map[string][]Source{}}
	for _, env := range envs {
		for _, v := range env.Vars {
			if _, seen := occ.sources[v.Key]; !seen {
				occ.order = append(occ.order, v.Key)
			}
			occ.sources[v.Key] = append(occ.sources[v.Key], Source{Env: env.Name, Value: v.Value})
		}
	}
	return occ
}

func (o occurrences) resolve(key string) (Resolved, bool) {
	sources, ok := o.sources[key]
	if !ok {
		return Resolved{}, false
	}
	winner := sources[len(sources)-1]
	shadows := make([]Source, len(sources)-1)
	copy(shadows, sources[:len(sources)-1])
	return Resolved{Key: key, Value: winner.Value, From: winner.Env, Shadows: shadows}, true
}

func (o occurrences) conflicts() []string {
	conflicts := []string{}
	for _, key := range o.order {
		if len(o.sources[key]) > 1 {
			conflicts = append(conflicts, key)
		}
	}
	return conflicts
}

func Resolve(envs []Env, tmpl *Template, opts Options) Plan {
	occ := collect(envs)
	plan := Plan{
		Envs:      envNames(envs),
		Vars:      []Resolved{},
		Conflicts: occ.conflicts(),
		Missing:   []string{},
		Extra:     []string{},
	}
	if tmpl == nil {
		for _, key := range occ.order {
			resolved, _ := occ.resolve(key)
			plan.Vars = append(plan.Vars, resolved)
		}
		return plan
	}
	inTemplate := make(map[string]struct{}, len(tmpl.Entries))
	for _, entry := range tmpl.Entries {
		inTemplate[entry.Key] = struct{}{}
		if resolved, ok := occ.resolve(entry.Key); ok {
			plan.Vars = append(plan.Vars, resolved)
			continue
		}
		if entry.HasDefault {
			plan.Vars = append(plan.Vars, Resolved{Key: entry.Key, Value: entry.Default, Shadows: []Source{}, Default: true})
			continue
		}
		plan.Missing = append(plan.Missing, entry.Key)
	}
	for _, key := range occ.order {
		if _, listed := inTemplate[key]; listed {
			continue
		}
		plan.Extra = append(plan.Extra, key)
		if !opts.OnlyTemplate {
			resolved, _ := occ.resolve(key)
			plan.Vars = append(plan.Vars, resolved)
		}
	}
	return plan
}

func envNames(envs []Env) []string {
	names := make([]string, len(envs))
	for i, env := range envs {
		names[i] = env.Name
	}
	return names
}

func MergeVars(existing, incoming []Var) []Var {
	updates := make(map[string]string, len(incoming))
	for _, v := range incoming {
		updates[v.Key] = v.Value
	}
	merged := make([]Var, 0, len(existing)+len(incoming))
	present := make(map[string]struct{}, len(existing))
	for _, v := range existing {
		present[v.Key] = struct{}{}
		if value, ok := updates[v.Key]; ok {
			v.Value = value
		}
		merged = append(merged, v)
	}
	for _, v := range incoming {
		if _, ok := present[v.Key]; ok {
			continue
		}
		present[v.Key] = struct{}{}
		merged = append(merged, v)
	}
	return merged
}
