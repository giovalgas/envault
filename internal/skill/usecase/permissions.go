package usecase

import "github.com/giovalgas/envault/internal/skill/domain"

func PermissionsJSON() (string, error) {
	return domain.PermissionsJSON()
}
