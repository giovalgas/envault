package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/vault/domain"
)

func TestInitVault(t *testing.T) {
	repo := &memRepo{snapshot: domain.NewSnapshot(baseTime)}
	uc := NewInitVault(repo)
	first, err := uc.Execute(context.Background())
	if err != nil || !first.Created || first.Location != "/mem/envault" {
		t.Fatalf("first = %+v, %v", first, err)
	}
	second, err := uc.Execute(context.Background())
	if err != nil || second.Created {
		t.Fatalf("second = %+v, %v", second, err)
	}
	repo.loadErr = domain.ErrDecrypt
	if _, err := uc.Execute(context.Background()); !errors.Is(err, domain.ErrDecrypt) {
		t.Fatalf("err = %v", err)
	}
}

func TestListEnvsFilters(t *testing.T) {
	repo := newRepo(
		domain.Env{Name: "stripe-test", Tags: []string{"payments"}, Vars: []domain.Var{{Key: "STRIPE_KEY", Value: "sk"}}},
		domain.Env{Name: "postgres-local", Description: "Banco local", Tags: []string{"db"}},
		domain.Env{Name: "aws-dev", Vars: []domain.Var{{Key: "AWS_REGION", Value: "us-east-1"}}},
	)
	uc := NewListEnvs(repo)
	cases := []struct {
		query ListEnvsQuery
		want  string
	}{
		{ListEnvsQuery{}, "aws-dev,postgres-local,stripe-test"},
		{ListEnvsQuery{Search: "POSTGRES"}, "postgres-local"},
		{ListEnvsQuery{Search: "banco"}, "postgres-local"},
		{ListEnvsQuery{Search: "DB"}, "postgres-local"},
		{ListEnvsQuery{Search: "region"}, "aws-dev"},
		{ListEnvsQuery{Tag: "payments"}, "stripe-test"},
		{ListEnvsQuery{Tag: "db", Search: "stripe"}, ""},
		{ListEnvsQuery{Search: "nada"}, ""},
	}
	for _, tc := range cases {
		envs, err := uc.Execute(context.Background(), tc.query)
		if err != nil {
			t.Fatalf("%+v: %v", tc.query, err)
		}
		if envs == nil {
			t.Fatalf("%+v: nil slice", tc.query)
		}
		names := make([]string, len(envs))
		for i, env := range envs {
			names[i] = env.Name
		}
		if got := strings.Join(names, ","); got != tc.want {
			t.Errorf("%+v = %s, want %s", tc.query, got, tc.want)
		}
	}
	repo.initialized = false
	if _, err := uc.Execute(context.Background(), ListEnvsQuery{}); !errors.Is(err, domain.ErrNotInitialized) {
		t.Fatalf("err = %v", err)
	}
}

func TestShowEnv(t *testing.T) {
	uc := NewShowEnv(newRepo(sampleEnv("a")))
	env, err := uc.Execute(context.Background(), "a")
	if err != nil || env.Name != "a" || !env.SameContent(sampleEnv("a")) {
		t.Fatalf("env = %+v, %v", env, err)
	}
	if _, err := uc.Execute(context.Background(), "nope"); !errors.Is(err, domain.ErrEnvNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestGetValue(t *testing.T) {
	uc := NewGetValue(newRepo(sampleEnv("a")))
	value, err := uc.Execute(context.Background(), "a", "POOL")
	if err != nil || value != "10" {
		t.Fatalf("value = %q, %v", value, err)
	}
	_, err = uc.Execute(context.Background(), "a", "NOPE")
	if !errors.Is(err, domain.ErrVarNotFound) || !strings.Contains(err.Error(), `"NOPE" em "a"`) {
		t.Fatalf("err = %v", err)
	}
	if _, err := uc.Execute(context.Background(), "b", "POOL"); !errors.Is(err, domain.ErrEnvNotFound) {
		t.Fatalf("err = %v", err)
	}
}
