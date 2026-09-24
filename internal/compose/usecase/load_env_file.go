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
	Envs         []string
	Template     TemplateView
	OnlyTemplate bool
	Target       string
	Dir          string
	Existing     ExistingTarget
}

type LoadEnvFileResult struct {
	Plan    PlanView
	Target  TargetView
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
	plan, err := combine(ctx, uc.envs, in.Envs, in.Template, in.OnlyTemplate)
	if err != nil {
		return LoadEnvFileResult{}, err
	}
	path := resolveIn(in.Dir, in.Target)
	exists, err := uc.files.Exists(path)
	if err != nil {
		return LoadEnvFileResult{}, err
	}
	if exists && in.Existing == RefuseExisting {
		return LoadEnvFileResult{}, ErrTargetExists
	}
	mode, written, err := uc.write(path, in.Existing, exists, plan.Pairs())
	if err != nil {
		return LoadEnvFileResult{}, err
	}
	target := domain.Target{Path: path, Exists: exists, Gitignored: uc.gitignore.IsIgnored(path)}
	return LoadEnvFileResult{Plan: planView(plan), Target: targetView(target), Mode: mode, Written: written}, nil
}

func (uc *LoadEnvFile) write(path string, existing ExistingTarget, exists bool, pairs []domain.Var) (LoadMode, int, error) {
	if exists && existing == MergeExisting {
		written, err := uc.files.Merge(path, pairs)
		return LoadMerged, written, err
	}
	mode := LoadCreated
	if exists {
		mode = LoadOverwritten
	}
	return mode, len(pairs), uc.files.Write(path, pairs)
}
