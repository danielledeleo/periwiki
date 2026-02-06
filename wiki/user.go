package wiki

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Role constants for user authorization.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// User represents a user in the wiki system.
type User struct {
	Email        string    `db:"email"`
	ScreenName   string    `db:"screenname"`
	ID           int       `db:"id"`
	PasswordHash string    `db:"passwordhash"`
	Role         string    `db:"role"`
	CreatedAt    time.Time `db:"created_at"`
	RawPassword  string
	IPAddress    string
}

// IsAnonymous returns true if the user is not authenticated.
func (u *User) IsAnonymous() bool {
	return u.ID == 0
}

// IsAdmin returns true if the user has the admin role.
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// SetPasswordHash generates and sets the bcrypt hash for the user's password.
func (u *User) SetPasswordHash() error {
	rawHash, err := bcrypt.GenerateFromPassword([]byte(u.RawPassword), bcrypt.MinCost)
	u.RawPassword = ""

	if err != nil {
		return err
	}

	u.PasswordHash = string(rawHash)
	return nil
}

// AnonymousUser returns an anonymous user with ID 0.
func AnonymousUser() *User {
	return &User{ID: 0}
}
