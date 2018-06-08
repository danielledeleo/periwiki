package main

import (
	"context"
	"net"
	"net/http"

	"github.com/jagger27/iwikii/model"
)

type key string

const userKey key = "User"

func (a *app) SessionMiddleware(handler http.Handler) http.Handler {

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		session, _ := a.GetCookie(req, "iwikii-login")
		if session.IsNew {
			anon := model.AnonymousUser()
			ip, _, _ := net.SplitHostPort(req.RemoteAddr)

			anon.ScreenName = "Anonymous"
			anon.IPAddress = ip

			ctx := context.WithValue(req.Context(), userKey, anon)
			handler.ServeHTTP(rw, req.WithContext(ctx))
			// Add some sort of "access denied context to req"
			return
		}
		screenname := session.Values["username"].(string)
		user, err := a.GetUserByScreenName(screenname)
		check(err)
		ctx := context.WithValue(req.Context(), userKey, user)
		handler.ServeHTTP(rw, req.WithContext(ctx))
	})
}
