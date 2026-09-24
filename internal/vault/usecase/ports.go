package usecase

import "github.com/giovalgas/envault/internal/vault/domain"

type (
	EnvRepository = domain.EnvRepository
	KeyStore      = domain.KeyStore
	Editor        = domain.Editor
	SessionEditor = domain.SessionEditor
	EditorProcess = domain.EditorProcess
	Clock         = domain.Clock
)

type EnvFileReader interface {
	ReadEnvFile(path string) ([]byte, error)
}
