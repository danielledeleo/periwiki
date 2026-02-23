package storage

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestSessionStore creates a SessionStore backed by an in-memory database.
func newTestSessionStore(t *testing.T, maxAge int, secret []byte) *SessionStore {
	t.Helper()
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	return NewSessionStore(db.conn, "/", maxAge, secret)
}

func TestSessionStore_SaveAndLoad(t *testing.T) {
	secret := []byte("test-secret-key-for-sessions-32b")
	store := newTestSessionStore(t, 86400, secret)

	// Create a new request and save a session with a value.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	session, err := store.New(req, "session")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !session.IsNew {
		t.Fatal("expected session.IsNew = true")
	}

	session.Values["username"] = "alice"
	if err := store.Save(req, w, session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// The response should contain a Set-Cookie header.
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected Set-Cookie header")
	}

	// Create a second request carrying the cookie, and load the session.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(cookies[0])

	loaded, err := store.New(req2, "session")
	if err != nil {
		t.Fatalf("New (load): %v", err)
	}
	if loaded.IsNew {
		t.Fatal("expected session.IsNew = false after load")
	}

	username, ok := loaded.Values["username"].(string)
	if !ok || username != "alice" {
		t.Fatalf("expected username 'alice', got %v", loaded.Values["username"])
	}
}

func TestSessionStore_ExpiredSession(t *testing.T) {
	secret := []byte("test-secret-key-for-sessions-32b")
	// MaxAge of 1 second.
	store := newTestSessionStore(t, 1, secret)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	session, err := store.New(req, "session")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	session.Values["username"] = "bob"
	if err := store.Save(req, w, session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cookies := w.Result().Cookies()

	// Wait for the session to expire.
	time.Sleep(1100 * time.Millisecond)

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(cookies[0])

	loaded, err := store.New(req2, "session")
	if err != nil {
		t.Fatalf("New (expired): %v", err)
	}
	if !loaded.IsNew {
		t.Fatal("expected expired session to be treated as new")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	secret := []byte("test-secret-key-for-sessions-32b")
	store := newTestSessionStore(t, 86400, secret)

	// Save a session.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	session, err := store.New(req, "session")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	session.Values["username"] = "charlie"
	if err := store.Save(req, w, session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cookies := w.Result().Cookies()

	// Delete the session.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(cookies[0])
	w2 := httptest.NewRecorder()

	// Reload first so we have a valid session.ID.
	session2, err := store.New(req2, "session")
	if err != nil {
		t.Fatalf("New (before delete): %v", err)
	}
	if session2.IsNew {
		t.Fatal("expected session to exist before delete")
	}

	if err := store.Delete(req2, w2, session2); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Load again — should be new.
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.AddCookie(cookies[0])

	loaded, err := store.New(req3, "session")
	if err != nil {
		t.Fatalf("New (after delete): %v", err)
	}
	if !loaded.IsNew {
		t.Fatal("expected deleted session to be treated as new")
	}
}

func TestSessionStore_InvalidCookie(t *testing.T) {
	secret := []byte("test-secret-key-for-sessions-32b")
	store := newTestSessionStore(t, 86400, secret)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "garbage-data"})

	session, err := store.New(req, "session")
	// securecookie decode error is returned (per gorilla convention),
	// but session should still be usable as a new session.
	if err == nil {
		t.Log("no error returned (cookie may have been silently ignored)")
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if !session.IsNew {
		t.Fatal("expected session.IsNew = true for invalid cookie")
	}
}

func TestSessionStore_DifferentSecret(t *testing.T) {
	secret1 := []byte("test-secret-key-for-sessions-32b")
	secret2 := []byte("completely-different-secret-key!")
	store1 := newTestSessionStore(t, 86400, secret1)

	// Save a session with store1.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	session, err := store1.New(req, "session")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	session.Values["username"] = "dave"
	if err := store1.Save(req, w, session); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cookies := w.Result().Cookies()

	// Create a second store with a different secret, same DB.
	store2 := NewSessionStore(store1.db, "/", 86400, secret2)

	// Try loading with the old cookie — the cookie decode should fail,
	// resulting in a new session (graceful degradation).
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(cookies[0])

	loaded, _ := store2.New(req2, "session")
	if !loaded.IsNew {
		t.Fatal("expected session.IsNew = true when cookie secret doesn't match")
	}
}
