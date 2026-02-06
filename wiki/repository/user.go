package repository

import "github.com/danielledeleo/periwiki/wiki"

// UserRepository defines the interface for user persistence operations.
type UserRepository interface {
	// SelectUserByScreenname retrieves a user by their screen name.
	// If withHash is true, includes the password hash in the result.
	SelectUserByScreenname(screenname string, withHash bool) (*wiki.User, error)

	// SelectUserByID retrieves a user by their ID.
	SelectUserByID(id int) (*wiki.User, error)

	// SelectAllUsers returns all users except anonymous (id != 0), ordered by id.
	SelectAllUsers() ([]*wiki.User, error)

	// InsertUser inserts a new user and populates user.ID with the new ID.
	InsertUser(user *wiki.User) error

	// UpdateUserRole updates a user's role. Returns an error if the role is invalid.
	UpdateUserRole(id int, role string) error
}
