package storage

import "github.com/danielledeleo/periwiki/wiki/repository"

// Compile-time checks that sqliteDb implements all repository interfaces.
var (
	_ repository.ArticleRepository    = (*sqliteDb)(nil)
	_ repository.UserRepository       = (*sqliteDb)(nil)
	_ repository.PreferenceRepository = (*sqliteDb)(nil)
	_ repository.LinkRepository       = (*sqliteDb)(nil)
	_ repository.SessionRepository    = (*sqliteDb)(nil)
)
