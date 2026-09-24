package compose

import (
	"context"

	"github.com/spf13/cobra"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
)

const (
	templateDefaultPath  = composeusecase.DefaultTemplateFile
	templateFlagPath     = "template"
	templateFlagDisabled = "no-template"
	templateFlagOnly     = "only-template"
)

type templateFlags struct {
	path     string
	disabled bool
	only     bool
}

func templateBind(cmd *cobra.Command) *templateFlags {
	flags := &templateFlags{}
	cmd.Flags().StringVar(&flags.path, templateFlagPath, "", "template de chaves (padrão: "+templateDefaultPath+" no diretório atual, quando existir)")
	cmd.Flags().BoolVar(&flags.disabled, templateFlagDisabled, false, "ignora qualquer template, inclusive o "+templateDefaultPath)
	cmd.Flags().BoolVar(&flags.only, templateFlagOnly, false, "grava só as chaves listadas no template")
	return flags
}

func (f *templateFlags) validate() error {
	if f.disabled && f.path != "" {
		return presenter.FlagsConflict(templateFlagPath, templateFlagDisabled)
	}
	if f.disabled && f.only {
		return presenter.FlagsConflict(templateFlagOnly, templateFlagDisabled)
	}
	return nil
}

func (f *templateFlags) resolve(ctx context.Context, compose app.ComposeUseCases) (composeusecase.TemplateView, error) {
	if err := f.validate(); err != nil {
		return composeusecase.TemplateView{}, err
	}
	tmpl, err := compose.LoadTemplate.Execute(ctx, composeusecase.LoadTemplateInput{Path: f.path, Disabled: f.disabled, Only: f.only})
	if err != nil {
		return composeusecase.TemplateView{}, presenter.TemplateError(err, f.path, templateFlagOnly, templateFlagPath)
	}
	return tmpl, nil
}
