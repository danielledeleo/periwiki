package main

import (
	"context"
	"net/http"
)

type key string

const userKey key = "User"

func (a *app) SessionMiddleware(handler http.Handler) http.Handler {

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		session, _ := a.GetCookie(req, "iwikii-login")
		if !session.IsNew {
			screenname := session.Values["username"].(string)
			user, err := a.GetUserByScreenName(screenname)
			check(err)
			ctx := context.WithValue(req.Context(), userKey, user)
			handler.ServeHTTP(rw, req.WithContext(ctx))
			return
		}
		handler.ServeHTTP(rw, req)
	})
}
