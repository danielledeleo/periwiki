package testutil

import "github.com/danielledeleo/periwiki/wiki/repository"

// Compile-time checks that TestDB implements all repository interfaces.
var (
	_ repository.ArticleRepository    = (*TestDB)(nil)
	_ repository.UserRepository       = (*TestDB)(nil)
	_ repository.PreferenceRepository = (*TestDB)(nil)
	_ repository.LinkRepository       = (*TestDB)(nil)
	_ repository.SessionRepository    = (*TestDB)(nil)
)
