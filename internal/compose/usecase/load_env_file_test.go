package usecase

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/giovalgas/envault/internal/compose/domain"
)

func loadTestRun(files *fakeFiles, existing ExistingTarget) (LoadEnvFileResult, *fakeGitignore, error) {
	ignore := &fakeGitignore{status: domain.GitignoreNotIgnored}
	uc := NewLoadEnvFile(newReader(env("a", "A", "1", "B", "2")), files, ignore)
	result, err := uc.Execute(context.Background(), LoadEnvFileInput{Envs: []string{"a"}, Target: ".env", Existing: existing})
	return result, ignore, err
}

func TestLoadEnvFileModes(t *testing.T) {
	cases := []struct {
		name     string
		exists   bool
		existing ExistingTarget
		mode     LoadMode
		written  int
		writes   int
		merges   int
	}{
		{name: "created", existing: RefuseExisting, mode: LoadCreated, written: 2, writes: 1},
		{name: "force without target creates", existing: ReplaceExisting, mode: LoadCreated, written: 2, writes: 1},
		{name: "merge without target creates", existing: MergeExisting, mode: LoadCreated, written: 2, writes: 1},
		{name: "overwritten", exists: true, existing: ReplaceExisting, mode: LoadOverwritten, written: 2, writes: 1},
		{name: "merged", exists: true, existing: MergeExisting, mode: LoadMerged, written: 5, merges: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := &fakeFiles{exists: tc.exists, mergeSize: 5}
			result, ignore, err := loadTestRun(files, tc.existing)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if result.Mode != tc.mode || result.Written != tc.written || files.writes != tc.writes || files.merges != tc.merges {
				t.Fatalf("result %+v writes %d merges %d", result, files.writes, files.merges)
			}
			sent := files.written
			if tc.merges > 0 {
				sent = files.merged
			}
			if !slices.Equal(sent, vars("A", "1", "B", "2")) {
				t.Fatalf("sent = %+v", sent)
			}
			want := TargetView{Path: ".env", Exists: tc.exists, Gitignored: domain.GitignoreNotIgnored}
			if result.Target != want || len(ignore.paths) != 1 {
				t.Fatalf("target = %+v paths %v", result.Target, ignore.paths)
			}
		})
	}
}

func TestLoadEnvFileRefusesExisting(t *testing.T) {
	files := &fakeFiles{exists: true}
	_, ignore, err := loadTestRun(files, RefuseExisting)
	if !errors.Is(err, ErrTargetExists) {
		t.Fatalf("err = %v", err)
	}
	if files.writes != 0 || files.merges != 0 || len(ignore.paths) != 0 {
		t.Fatalf("side effects: writes %d merges %d gitignore %v", files.writes, files.merges, ignore.paths)
	}
}

func TestLoadEnvFileErrors(t *testing.T) {
	cases := map[string]*fakeFiles{
		"exists": {existsErr: errBoom},
		"write":  {writeErr: errBoom},
		"merge":  {exists: true, mergeErr: errBoom},
	}
	for name, files := range cases {
		existing := RefuseExisting
		if files.exists {
			existing = MergeExisting
		}
		if _, _, err := loadTestRun(files, existing); !errors.Is(err, errBoom) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	uc := NewLoadEnvFile(newReader(), &fakeFiles{}, &fakeGitignore{})
	if _, err := uc.Execute(context.Background(), LoadEnvFileInput{Envs: []string{"x", "x"}}); err == nil {
		t.Fatal("duplicate env accepted")
	}
	var missing *EnvNotFoundError
	if _, err := uc.Execute(context.Background(), LoadEnvFileInput{Envs: []string{"x"}}); !errors.As(err, &missing) {
		t.Fatalf("missing env err = %v", err)
	}
}
