package usecase

import (
	"context"
	"fmt"

	"github.com/giovalgas/envault/internal/shared/dotenv"
)

const DefaultTemplateFile = ".env.example"

type LoadTemplateInput struct {
	Path     string
	Dir      string
	Disabled bool
	Only     bool
}

type LoadTemplate struct {
	files TemplateReader
}

func NewLoadTemplate(files TemplateReader) *LoadTemplate {
	return &LoadTemplate{files: files}
}

func (uc *LoadTemplate) Execute(_ context.Context, in LoadTemplateInput) (TemplateView, error) {
	if in.Disabled {
		return TemplateView{}, nil
	}
	explicit := in.Path != ""
	path := in.Path
	if !explicit {
		path = DefaultTemplateFile
	}
	data, found, err := uc.files.ReadTemplate(resolveIn(in.Dir, path))
	switch {
	case err != nil:
		return TemplateView{}, &TemplateReadError{Path: path, Err: err}
	case !found && explicit:
		return TemplateView{}, fmt.Errorf("%w: %s", ErrTemplateNotFound, path)
	case !found && in.Only:
		return TemplateView{}, ErrTemplateRequired
	case !found:
		return TemplateView{Path: path}, nil
	}
	parsed, err := dotenv.ParseTemplate(data)
	if err != nil {
		return TemplateView{}, &TemplateParseError{Path: path, Err: err}
	}
	entries := make([]TemplateEntryView, len(parsed.Entries))
	for i, entry := range parsed.Entries {
		entries[i] = TemplateEntryView(entry)
	}
	return TemplateView{Path: path, Found: true, Entries: entries}, nil
}
