package usecase

import (
	"github.com/giovalgas/envault/internal/shared/dotenv"
	"github.com/giovalgas/envault/internal/vault/domain"
)

var (
	ErrNotInitialized     = domain.ErrNotInitialized
	ErrEnvNotFound        = domain.ErrEnvNotFound
	ErrEnvExists          = domain.ErrEnvExists
	ErrDecrypt            = domain.ErrDecrypt
	ErrInvalidName        = domain.ErrInvalidName
	ErrInvalidKey         = domain.ErrInvalidKey
	ErrInvalidTag         = domain.ErrInvalidTag
	ErrInvalidDescription = domain.ErrInvalidDescription
	ErrUnsupportedSchema  = domain.ErrUnsupportedSchema
	ErrVarNotFound        = domain.ErrVarNotFound
	ErrEditConflict       = domain.ErrEditConflict
	ErrEditCanceled       = domain.ErrEditCanceled
	ErrEditReopen         = domain.ErrEditReopen
	ErrMalformedKey       = domain.ErrMalformedKey
	ErrSyntax             = dotenv.ErrSyntax
)

func ValidateName(name string) error {
	return domain.ValidateName(name)
}
