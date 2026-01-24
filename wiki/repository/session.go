package repository

import (
	"net/http"

	"github.com/gorilla/sessions"
)

// SessionRepository defines the interface for session persistence operations.
// It wraps the gorilla/sessions.Store interface with an additional Delete method.
type SessionRepository interface {
	sessions.Store

	// Delete removes a session.
	Delete(r *http.Request, rw http.ResponseWriter, s *sessions.Session) error
}
