package server

import (
	"net/http"
	"net/url"

	"github.com/danielledeleo/periwiki/wiki"
)

// RequireAuth returns the authenticated user, or redirects to login and returns nil.
func (a *App) RequireAuth(rw http.ResponseWriter, req *http.Request) *wiki.User {
	user := req.Context().Value(wiki.UserKey).(*wiki.User)
	if user.IsAnonymous() {
		loginURL := "/user/login?reason=login_required&referrer=" + url.QueryEscape(req.URL.String())
		http.Redirect(rw, req, loginURL, http.StatusSeeOther)
		return nil
	}
	return user
}

// RequireAdmin returns the user if they are an admin, or shows an appropriate
// error and returns nil. Anonymous users are redirected to login; authenticated
// non-admins get a 403 page.
func (a *App) RequireAdmin(rw http.ResponseWriter, req *http.Request) *wiki.User {
	user := a.RequireAuth(rw, req)
	if user == nil {
		return nil
	}
	if !user.IsAdmin() {
		a.ErrorHandler(http.StatusForbidden, rw, req, wiki.ErrAdminRequired)
		return nil
	}
	return user
}
