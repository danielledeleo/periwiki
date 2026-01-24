package service

import (
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/repository"
)

// PreferenceService defines the interface for preference operations.
type PreferenceService interface {
	// GetPreference retrieves a preference by its key.
	GetPreference(key string) (*wiki.Preference, error)

	// UpdatePreference updates a preference.
	UpdatePreference(pref *wiki.Preference) error
}

// preferenceService is the default implementation of PreferenceService.
type preferenceService struct {
	repo repository.PreferenceRepository
}

// NewPreferenceService creates a new PreferenceService.
func NewPreferenceService(repo repository.PreferenceRepository) PreferenceService {
	return &preferenceService{repo: repo}
}

// GetPreference retrieves a preference by its key.
func (s *preferenceService) GetPreference(key string) (*wiki.Preference, error) {
	pref, err := s.repo.SelectPreference(key)
	if err != nil {
		return nil, err
	}

	return pref, err
}

// UpdatePreference updates a preference.
func (s *preferenceService) UpdatePreference(pref *wiki.Preference) error {
	return s.repo.InsertPreference(pref)
}
