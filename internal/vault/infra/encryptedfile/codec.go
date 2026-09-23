package encryptedfile

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/giovalgas/envault/internal/vault/domain"
)

const SchemaVersion = 1

type snapshotDocument struct {
	SchemaVersion int                    `json:"schema_version"`
	UpdatedAt     time.Time              `json:"updated_at"`
	Envs          map[string]envDocument `json:"envs"`
}

type envDocument struct {
	Description string        `json:"description"`
	Tags        []string      `json:"tags"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Vars        []varDocument `json:"vars"`
}

type varDocument struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func decode(plaintext []byte) (*domain.Snapshot, error) {
	var doc snapshotDocument
	if err := json.Unmarshal(plaintext, &doc); err != nil {
		return nil, fmt.Errorf("%w: conteúdo corrompido", domain.ErrDecrypt)
	}
	if doc.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("%w: encontrada %d, suportada até %d", domain.ErrUnsupportedSchema, doc.SchemaVersion, SchemaVersion)
	}
	if doc.SchemaVersion < 1 {
		return nil, fmt.Errorf("%w: schema_version ausente", domain.ErrDecrypt)
	}
	snapshot := domain.NewSnapshot(doc.UpdatedAt)
	for name, env := range doc.Envs {
		snapshot.Envs[name] = env.toDomain(name)
	}
	return snapshot, nil
}

func encode(snapshot *domain.Snapshot) ([]byte, error) {
	doc := snapshotDocument{
		SchemaVersion: SchemaVersion,
		UpdatedAt:     snapshot.UpdatedAt,
		Envs:          make(map[string]envDocument, len(snapshot.Envs)),
	}
	for name, env := range snapshot.Envs {
		doc.Envs[name] = envFromDomain(env)
	}
	return json.Marshal(doc)
}

func envFromDomain(env domain.Env) envDocument {
	doc := envDocument{
		Description: env.Description,
		Tags:        append([]string{}, env.Tags...),
		CreatedAt:   env.CreatedAt,
		UpdatedAt:   env.UpdatedAt,
		Vars:        make([]varDocument, len(env.Vars)),
	}
	for i, v := range env.Vars {
		doc.Vars[i] = varDocument(v)
	}
	return doc
}

func (d envDocument) toDomain(name string) domain.Env {
	env := domain.Env{
		Name:        name,
		Description: d.Description,
		Tags:        d.Tags,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	if d.Vars != nil {
		env.Vars = make([]domain.Var, len(d.Vars))
		for i, v := range d.Vars {
			env.Vars[i] = domain.Var(v)
		}
	}
	return env
}
