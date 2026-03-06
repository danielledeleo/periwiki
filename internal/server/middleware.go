package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/danielledeleo/periwiki/wiki"
)

// PrintModeMiddleware checks for the ?print query parameter and stores it in context.
func PrintModeMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, printMode := req.URL.Query()["print"]
		ctx := context.WithValue(req.Context(), wiki.PrintModeKey, printMode)
		handler.ServeHTTP(rw, req.WithContext(ctx))
	})
}

func (a *App) SessionMiddleware(handler http.Handler) http.Handler {

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		session, err := a.Sessions.GetCookie(req, "periwiki-login")
		check(err)
		// Helper to serve as anonymous user
		serveAsAnonymous := func() {
			anon := wiki.AnonymousUser()
			ip, _, _ := net.SplitHostPort(req.RemoteAddr)
			anon.ScreenName = "Anonymous"
			anon.IPAddress = ip
			ctx := context.WithValue(req.Context(), wiki.UserKey, anon)
			handler.ServeHTTP(rw, req.WithContext(ctx))
		}

		if session.IsNew {
			serveAsAnonymous()
			return
		}

		// Safe type assertion to prevent panic on corrupted session
		screenname, ok := session.Values["username"].(string)
		if !ok || screenname == "" {
			serveAsAnonymous()
			return
		}

		user, err := a.Users.GetUserByScreenName(screenname)
		if err != nil || user == nil {
			// User not found in database, treat as anonymous
			serveAsAnonymous()
			return
		}

		slog.Debug("session authenticated", "category", "auth", "username", screenname)
		ctx := context.WithValue(req.Context(), wiki.UserKey, user)
		handler.ServeHTTP(rw, req.WithContext(ctx))
	})
}
