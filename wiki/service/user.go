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

	return s.repo.InsertUser(user)
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
