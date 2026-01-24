package service

import (
	"net/http"

	"github.com/danielledeleo/periwiki/wiki/repository"
	"github.com/gorilla/sessions"
)

// SessionService defines the interface for session management operations.
type SessionService interface {
	// GetCookie retrieves an existing session by name.
	GetCookie(r *http.Request, name string) (*sessions.Session, error)

	// NewCookie creates a new session with the given name.
	NewCookie(r *http.Request, name string) (*sessions.Session, error)

	// SaveCookie saves a session to the response.
	SaveCookie(r *http.Request, rw http.ResponseWriter, s *sessions.Session) error

	// DeleteCookie removes a session.
	DeleteCookie(r *http.Request, rw http.ResponseWriter, s *sessions.Session) error
}

// sessionService is the default implementation of SessionService.
type sessionService struct {
	repo repository.SessionRepository
}

// NewSessionService creates a new SessionService.
func NewSessionService(repo repository.SessionRepository) SessionService {
	return &sessionService{repo: repo}
}

// GetCookie retrieves an existing session by name.
func (s *sessionService) GetCookie(r *http.Request, name string) (*sessions.Session, error) {
	return s.repo.Get(r, name)
}

// NewCookie creates a new session with the given name.
func (s *sessionService) NewCookie(r *http.Request, name string) (*sessions.Session, error) {
	return s.repo.New(r, name)
}

// SaveCookie saves a session to the response.
func (s *sessionService) SaveCookie(r *http.Request, rw http.ResponseWriter, s2 *sessions.Session) error {
	return s.repo.Save(r, rw, s2)
}

// DeleteCookie removes a session.
func (s *sessionService) DeleteCookie(r *http.Request, rw http.ResponseWriter, s2 *sessions.Session) error {
	return s.repo.Delete(r, rw, s2)
}
