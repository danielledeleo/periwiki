package wiki

import "errors"

// Sentinel errors for wiki operations
var (
	ErrUsernameTaken         = errors.New("username already in use")
	ErrEmailTaken            = errors.New("email already in use")
	ErrPasswordTooShort      = errors.New("password too short")
	ErrIncorrectPassword     = errors.New("incorrect password")
	ErrUsernameNotFound      = errors.New("username not found")
	ErrBadUsername           = errors.New("username must only contain letters, numbers, -, or _")
	ErrEmptyUsername         = errors.New("username cannot be empty")
	ErrArticleNotModified    = errors.New("article not modified")
	ErrRevisionNotFound      = errors.New("revision not found")
	ErrRevisionAlreadyExists = errors.New("revision already exists")
	ErrGenericNotFound       = errors.New("not found")
	ErrNoArticles            = errors.New("no articles exist")
	ErrReadOnlyArticle       = errors.New("article is read-only")
	ErrForbidden             = errors.New("you do not have permission to perform this action")
	ErrAdminRequired         = errors.New("administrator access required")
)
