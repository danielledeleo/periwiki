package repository

import "github.com/danielledeleo/periwiki/wiki"

// PreferenceRepository defines the interface for preference persistence operations.
type PreferenceRepository interface {
	// SelectPreference retrieves a preference by its key.
	SelectPreference(key string) (*wiki.Preference, error)

	// InsertPreference inserts or updates a preference.
	InsertPreference(pref *wiki.Preference) error
}
