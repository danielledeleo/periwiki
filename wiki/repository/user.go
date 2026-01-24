package repository

import "github.com/danielledeleo/periwiki/wiki"

// UserRepository defines the interface for user persistence operations.
type UserRepository interface {
	// SelectUserByScreenname retrieves a user by their screen name.
	// If withHash is true, includes the password hash in the result.
	SelectUserByScreenname(screenname string, withHash bool) (*wiki.User, error)

	// InsertUser inserts a new user.
	InsertUser(user *wiki.User) error
}
