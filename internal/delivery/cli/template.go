package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	composedomain "github.com/giovalgas/envault/internal/compose/domain"
	"github.com/giovalgas/envault/internal/shared/dotenv"
)

const (
	templateDefaultPath    = ".env.example"
	templateFlagPath       = "template"
	templateFlagDisabled   = "no-template"
	templateFlagOnly       = "only-template"
	templateNoTemplateHint = "--only-template exige um template: passe --template ou crie " + templateDefaultPath
)

type templateFlags struct {
	path     string
	disabled bool
	only     bool
}

type templateSource struct {
	Path     string
	Found    bool
	Template *composedomain.Template
}

func (f *templateFlags) options() composedomain.Options {
	return composedomain.Options{OnlyTemplate: f.only}
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
		return usageError(fmt.Errorf("--%s e --%s não podem ser usados juntos", templateFlagPath, templateFlagDisabled))
	}
	if f.disabled && f.only {
		return usageError(fmt.Errorf("--%s e --%s não podem ser usados juntos", templateFlagOnly, templateFlagDisabled))
	}
	return nil
}

func (f *templateFlags) resolve() (templateSource, error) {
	if err := f.validate(); err != nil {
		return templateSource{}, err
	}
	if f.disabled {
		return templateSource{}, nil
	}
	explicit := f.path != ""
	path := f.path
	if !explicit {
		path = templateDefaultPath
	}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist) && explicit:
		return templateSource{}, fmt.Errorf("%w: template %s não existe", ErrValidation, path)
	case errors.Is(err, fs.ErrNotExist) && f.only:
		return templateSource{}, fmt.Errorf("%w: %s", ErrValidation, templateNoTemplateHint)
	case errors.Is(err, fs.ErrNotExist):
		return templateSource{Path: path}, nil
	case err != nil:
		return templateSource{}, fmt.Errorf("ler template %s: %w", path, err)
	}
	parsed, err := dotenv.ParseTemplate(data)
	if err != nil {
		return templateSource{}, fmt.Errorf("template %s: %w", path, err)
	}
	tmpl := templateConvert(parsed)
	return templateSource{Path: path, Found: true, Template: &tmpl}, nil
}

func templateConvert(parsed dotenv.Template) composedomain.Template {
	entries := make([]composedomain.TemplateEntry, len(parsed.Entries))
	for i, entry := range parsed.Entries {
		entries[i] = composedomain.TemplateEntry{Key: entry.Key, Default: entry.Default, HasDefault: entry.HasDefault}
	}
	return composedomain.Template{Entries: entries}
}
