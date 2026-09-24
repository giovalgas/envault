package domain

import (
	"errors"

	"github.com/giovalgas/envault/internal/shared/dotenv"
)

var (
	ErrNotInitialized     = errors.New("cofre não inicializado")
	ErrEnvNotFound        = errors.New("env não encontrada")
	ErrEnvExists          = errors.New("env já existe")
	ErrDecrypt            = errors.New("falha ao decifrar o cofre")
	ErrInvalidName        = errors.New("nome de env inválido")
	ErrInvalidKey         = dotenv.ErrInvalidKey
	ErrInvalidTag         = dotenv.ErrInvalidTag
	ErrInvalidDescription = errors.New("descrição inválida")
	ErrUnsupportedSchema  = errors.New("versão de schema não suportada")
	ErrVarNotFound        = errors.New("chave não encontrada")
	ErrEditConflict       = errors.New("a env mudou no cofre durante a edição; nada foi gravado")
	ErrEditCanceled       = errors.New("edição cancelada")
	ErrEditReopen         = errors.New("conteúdo inválido, reabra o editor")
)

var (
	ErrNoKey              = errors.New("chave não encontrada")
	ErrKeyExists          = errors.New("chave já existe")
	ErrMalformedKey       = errors.New("chave inválida")
	ErrKeyringUnavailable = errors.New("keychain do sistema indisponível")
)
