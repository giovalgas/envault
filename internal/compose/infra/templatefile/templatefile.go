package templatefile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/giovalgas/envault/internal/compose/usecase"
)

type Reader struct{}

var _ usecase.TemplateReader = Reader{}

func New() Reader {
	return Reader{}
}

func (Reader) ReadTemplate(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	return data, true, nil
}
