package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type ExistingTarget int

const (
	RefuseExisting ExistingTarget = iota
	ReplaceExisting
	MergeExisting
)

type LoadMode string

const (
	LoadCreated     LoadMode = "created"
	LoadOverwritten LoadMode = "overwritten"
	LoadMerged      LoadMode = "merged"
)

type LoadEnvFileInput struct {
	Envs     []string
	Template *domain.Template
	Options  domain.Options
	Target   string
	Existing ExistingTarget
}

type LoadEnvFileResult struct {
	Plan    domain.Plan
	Target  domain.Target
	Mode    LoadMode
	Written int
}

type LoadEnvFile struct {
	envs      EnvReader
	files     TargetFiles
	gitignore GitignoreChecker
}

func NewLoadEnvFile(envs EnvReader, files TargetFiles, gitignore GitignoreChecker) *LoadEnvFile {
	return &LoadEnvFile{envs: envs, files: files, gitignore: gitignore}
}

func (uc *LoadEnvFile) Execute(ctx context.Context, in LoadEnvFileInput) (LoadEnvFileResult, error) {
	plan, err := combine(ctx, uc.envs, in.Envs, in.Template, in.Options)
	if err != nil {
		return LoadEnvFileResult{}, err
	}
	exists, err := uc.files.Exists(in.Target)
	if err != nil {
		return LoadEnvFileResult{}, err
	}
	if exists && in.Existing == RefuseExisting {
		return LoadEnvFileResult{}, ErrTargetExists
	}
	mode, written, err := uc.write(in, exists, plan.Pairs())
	if err != nil {
		return LoadEnvFileResult{}, err
	}
	target := domain.Target{Path: in.Target, Exists: exists, Gitignored: uc.gitignore.IsIgnored(in.Target)}
	return LoadEnvFileResult{Plan: plan, Target: target, Mode: mode, Written: written}, nil
}

func (uc *LoadEnvFile) write(in LoadEnvFileInput, exists bool, pairs []domain.Var) (LoadMode, int, error) {
	if exists && in.Existing == MergeExisting {
		written, err := uc.files.Merge(in.Target, pairs)
		return LoadMerged, written, err
	}
	mode := LoadCreated
	if exists {
		mode = LoadOverwritten
	}
	return mode, len(pairs), uc.files.Write(in.Target, pairs)
}
