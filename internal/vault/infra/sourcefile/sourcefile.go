package sourcefile

import (
	"os"
	"path/filepath"

	"github.com/giovalgas/envault/internal/vault/usecase"
)

type Reader struct{}

var _ usecase.EnvFileReader = Reader{}

func New() Reader {
	return Reader{}
}

func (Reader) ReadEnvFile(path string) ([]byte, error) {
	return os.ReadFile(filepath.Clean(path))
}
