package service

import (
	"github.com/danielledeleo/periwiki/wiki/repository"
)

// PreferenceService defines the interface for preference operations.
type PreferenceService interface {
}

// preferenceService is the default implementation of PreferenceService.
type preferenceService struct {
	repo repository.PreferenceRepository
}

// NewPreferenceService creates a new PreferenceService.
func NewPreferenceService(repo repository.PreferenceRepository) PreferenceService {
	return &preferenceService{repo: repo}
}

