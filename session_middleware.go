package main

import (
	"context"
	"net"
	"net/http"

	"github.com/jagger27/iwikii/model"
)

func (a *app) SessionMiddleware(handler http.Handler) http.Handler {

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		session, err := a.GetCookie(req, "iwikii-login")
		check(err)
		if session.IsNew {
			anon := model.AnonymousUser()
			ip, _, _ := net.SplitHostPort(req.RemoteAddr)

			anon.ScreenName = "Anonymous"
			anon.IPAddress = ip

			ctx := context.WithValue(req.Context(), model.UserKey, anon)
			handler.ServeHTTP(rw, req.WithContext(ctx))
			// Add some sort of "access denied context to req"
			return
		}
		screenname := session.Values["username"].(string)
		user, err := a.GetUserByScreenName(screenname)
		check(err)
		ctx := context.WithValue(req.Context(), model.UserKey, user)
		handler.ServeHTTP(rw, req.WithContext(ctx))
	})
}
