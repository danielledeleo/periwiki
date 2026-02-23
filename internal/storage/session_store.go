package storage

import (
	"encoding/gob"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
)

func init() {
	gob.Register(time.Time{})
}

// SessionStore is a gorilla/sessions backend backed by SQLite.
// It replaces the unmaintained michaeljs1990/sqlitestore package,
// reusing the app's existing *sqlx.DB (modernc.org/sqlite, pure Go).
//
// Wire-compatible with sqlitestore: the cookie carries a securecookie-encoded
// integer session ID; the session payload is a separate securecookie-encoded
// blob in the session_data column.
type SessionStore struct {
	db     *sqlx.DB
	codecs []securecookie.Codec
	Options *sessions.Options
}

// NewSessionStore creates a SessionStore. keyPairs are passed to
// securecookie.CodecsFromPairs (hash key, optional encrypt key, …).
func NewSessionStore(db *sqlx.DB, path string, maxAge int, keyPairs ...[]byte) *SessionStore {
	return &SessionStore{
		db:     db,
		codecs: securecookie.CodecsFromPairs(keyPairs...),
		Options: &sessions.Options{
			Path:   path,
			MaxAge: maxAge,
		},
	}
}

// Get returns a cached session if present, otherwise calls New.
func (s *SessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New creates or loads a session. Decode errors from the cookie are returned
// (so callers can detect tampering), but DB load errors are silently swallowed
// — matching gorilla store convention (treat as "new session").
func (s *SessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	session.Options = &sessions.Options{
		Path:   s.Options.Path,
		MaxAge: s.Options.MaxAge,
	}
	session.IsNew = true

	var err error
	if c, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, c.Value, &session.ID, s.codecs...)
		if err == nil {
			if loadErr := s.load(session); loadErr == nil {
				session.IsNew = false
			}
			// DB/expiry errors → treat as new session, no error returned
		}
	}
	return session, err
}

// Save persists the session to the database and sets the cookie.
func (s *SessionStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	var err error
	if session.ID == "" {
		err = s.insert(session)
	} else {
		err = s.save(session)
	}
	if err != nil {
		return err
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, s.codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

// Delete removes the session from the database and expires the cookie.
func (s *SessionStore) Delete(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	options := *session.Options
	options.MaxAge = -1
	http.SetCookie(w, sessions.NewCookie(session.Name(), "", &options))

	for k := range session.Values {
		delete(session.Values, k)
	}

	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, session.ID)
	return err
}

// insert creates a new session row and sets session.ID.
func (s *SessionStore) insert(session *sessions.Session) error {
	createdOn := time.Now()
	expiresOn := createdOn.Add(time.Second * time.Duration(session.Options.MaxAge))

	// Strip internal time keys (matches sqlitestore behaviour).
	delete(session.Values, "created_on")
	delete(session.Values, "expires_on")
	delete(session.Values, "modified_on")

	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, s.codecs...)
	if err != nil {
		return err
	}

	res, err := s.db.Exec(
		`INSERT INTO sessions (id, session_data, created_on, expires_on) VALUES (NULL, ?, ?, ?)`,
		encoded, createdOn, expiresOn,
	)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	session.ID = fmt.Sprintf("%d", id)
	return nil
}

// save updates an existing session row.
func (s *SessionStore) save(session *sessions.Session) error {
	if session.IsNew {
		return s.insert(session)
	}

	expiresOn := time.Now().Add(time.Second * time.Duration(session.Options.MaxAge))

	delete(session.Values, "created_on")
	delete(session.Values, "expires_on")
	delete(session.Values, "modified_on")

	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, s.codecs...)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		`UPDATE sessions SET session_data = ?, expires_on = ? WHERE id = ?`,
		encoded, expiresOn, session.ID,
	)
	return err
}

// load reads a session row from the database. Returns an error if the row
// doesn't exist or has expired.
func (s *SessionStore) load(session *sessions.Session) error {
	var data string
	var expiresOn time.Time

	err := s.db.QueryRow(
		`SELECT session_data, expires_on FROM sessions WHERE id = ?`, session.ID,
	).Scan(&data, &expiresOn)
	if err != nil {
		return err
	}

	if time.Now().After(expiresOn) {
		return fmt.Errorf("session expired")
	}

	return securecookie.DecodeMulti(session.Name(), data, &session.Values, s.codecs...)
}
