package usecase

import (
	"context"
)

type LoadShellExportsInput struct {
	Envs         []string
	Template     TemplateView
	OnlyTemplate bool
	Dialect      string
	ExportFile   string
}

type LoadShellExportsResult struct {
	Plan    PlanView
	Written int
}

type LoadShellExports struct {
	render  *RenderShell
	exports ExportWriter
}

func NewLoadShellExports(render *RenderShell, exports ExportWriter) *LoadShellExports {
	return &LoadShellExports{render: render, exports: exports}
}

func (uc *LoadShellExports) Execute(ctx context.Context, in LoadShellExportsInput) (LoadShellExportsResult, error) {
	if in.ExportFile == "" {
		return LoadShellExportsResult{}, ErrNoExportFile
	}
	rendered, err := uc.render.Execute(ctx, RenderShellInput{
		Envs:         in.Envs,
		Dialect:      in.Dialect,
		Template:     in.Template,
		OnlyTemplate: in.OnlyTemplate,
	})
	if err != nil {
		return LoadShellExportsResult{}, err
	}
	if err := uc.exports.WriteExports(in.ExportFile, rendered.Script); err != nil {
		return LoadShellExportsResult{}, err
	}
	return LoadShellExportsResult{Plan: rendered.Plan, Written: len(rendered.Plan.Vars)}, nil
}
