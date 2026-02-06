package service

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/repository"
	"golang.org/x/crypto/bcrypt"
)

// UserService defines the interface for user management operations.
type UserService interface {
	// PostUser creates a new user after validation.
	PostUser(user *wiki.User) error

	// CheckUserPassword verifies a user's password.
	CheckUserPassword(user *wiki.User) error

	// GetUserByScreenName retrieves a user by their screen name.
	GetUserByScreenName(screenname string) (*wiki.User, error)

	// GetAllUsers returns all registered users (excluding anonymous).
	GetAllUsers() ([]*wiki.User, error)

	// GetUserByID retrieves a user by their ID.
	GetUserByID(id int) (*wiki.User, error)

	// SetUserRole changes a user's role. The acting user must be an admin.
	SetUserRole(actingUser *wiki.User, targetID int, role string) error
}

// userService is the default implementation of UserService.
type userService struct {
	repo                  repository.UserRepository
	minimumPasswordLength int
}

// NewUserService creates a new UserService.
func NewUserService(repo repository.UserRepository, minimumPasswordLength int) UserService {
	return &userService{
		repo:                  repo,
		minimumPasswordLength: minimumPasswordLength,
	}
}

// PostUser creates a new user after validation.
// If the newly created user has ID 1, they are automatically promoted to admin.
func (s *userService) PostUser(user *wiki.User) error {
	if len(user.ScreenName) == 0 {
		return wiki.ErrEmptyUsername
	}

	matched, err := regexp.MatchString(`^[\p{L}0-9-_]+$`, user.ScreenName)
	if err != nil {
		return err
	}

	if !matched {
		return wiki.ErrBadUsername
	}

	if len(user.RawPassword) < s.minimumPasswordLength {
		return errors.New(wiki.ErrPasswordTooShort.Error() + fmt.Sprintf(" (must be %d characters long)", s.minimumPasswordLength))
	}

	err = user.SetPasswordHash()
	if err != nil {
		return err
	}

	if err := s.repo.InsertUser(user); err != nil {
		return err
	}

	// Promote first registered user to admin
	if user.ID == 1 {
		if err := s.repo.UpdateUserRole(1, wiki.RoleAdmin); err != nil {
			return err
		}
		user.Role = wiki.RoleAdmin
	}

	return nil
}

// CheckUserPassword verifies a user's password.
func (s *userService) CheckUserPassword(u *wiki.User) error {
	dbUser, err := s.repo.SelectUserByScreenname(u.ScreenName, true)
	if err == sql.ErrNoRows {
		return wiki.ErrUsernameNotFound
	}
	if err != nil {
		return err
	}

	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(u.RawPassword))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return wiki.ErrIncorrectPassword
	}

	return err
}

// GetUserByScreenName retrieves a user by their screen name.
func (s *userService) GetUserByScreenName(screenname string) (*wiki.User, error) {
	dbUser, err := s.repo.SelectUserByScreenname(screenname, false)
	if err == sql.ErrNoRows {
		return nil, wiki.ErrUsernameNotFound
	}

	return dbUser, err
}

// GetAllUsers returns all registered users (excluding anonymous).
func (s *userService) GetAllUsers() ([]*wiki.User, error) {
	return s.repo.SelectAllUsers()
}

// GetUserByID retrieves a user by their ID.
func (s *userService) GetUserByID(id int) (*wiki.User, error) {
	user, err := s.repo.SelectUserByID(id)
	if err == sql.ErrNoRows {
		return nil, wiki.ErrGenericNotFound
	}
	return user, err
}

// SetUserRole changes a user's role. The acting user must be an admin
// and cannot demote themselves.
func (s *userService) SetUserRole(actingUser *wiki.User, targetID int, role string) error {
	if !actingUser.IsAdmin() {
		return wiki.ErrAdminRequired
	}
	if role != wiki.RoleAdmin && role != wiki.RoleUser {
		return fmt.Errorf("invalid role: %s", role)
	}
	if actingUser.ID == targetID {
		return wiki.ErrForbidden
	}
	return s.repo.UpdateUserRole(targetID, role)
}
