package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/danielledeleo/periwiki/wiki"
)

func (a *app) SessionMiddleware(handler http.Handler) http.Handler {

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		session, err := a.GetCookie(req, "periwiki-login")
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

		user, err := a.GetUserByScreenName(screenname)
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
