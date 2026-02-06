package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/gorilla/mux"
)

// ManageUsersHandler lists all users for admin management.
func (a *App) ManageUsersHandler(rw http.ResponseWriter, req *http.Request) {
	user := a.RequireAdmin(rw, req)
	if user == nil {
		return
	}

	users, err := a.Users.GetAllUsers()
	if err != nil {
		a.ErrorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}

	data := map[string]any{
		"Page":    wiki.NewStaticPage("Manage Users"),
		"Context": req.Context(),
		"Users":   users,
	}

	if msg := req.URL.Query().Get("msg"); msg != "" {
		data["calloutMessage"] = msg
		data["calloutClasses"] = "pw-success"
	}
	if errMsg := req.URL.Query().Get("err"); errMsg != "" {
		data["calloutMessage"] = errMsg
		data["calloutClasses"] = "pw-error"
	}

	err = a.RenderTemplate(rw, "users.html", "index.html", data)
	check(err)
}

// ManageUserRoleHandler handles POST requests to change a user's role.
func (a *App) ManageUserRoleHandler(rw http.ResponseWriter, req *http.Request) {
	user := a.RequireAdmin(rw, req)
	if user == nil {
		return
	}

	vars := mux.Vars(req)
	targetID, err := strconv.Atoi(vars["id"])
	if err != nil {
		a.ErrorHandler(http.StatusBadRequest, rw, req, err)
		return
	}

	role := req.PostFormValue("role")

	if err := a.Users.SetUserRole(user, targetID, role); err != nil {
		slog.Warn("role change failed", "acting_user", user.ScreenName, "target_id", targetID, "role", role, "error", err)
		http.Redirect(rw, req, "/manage/users?err="+err.Error(), http.StatusSeeOther)
		return
	}

	slog.Info("user role changed", "acting_user", user.ScreenName, "target_id", targetID, "new_role", role)
	http.Redirect(rw, req, "/manage/users?msg=Role+updated", http.StatusSeeOther)
}

// ManageSettingsHandler displays the runtime settings form.
func (a *App) ManageSettingsHandler(rw http.ResponseWriter, req *http.Request) {
	user := a.RequireAdmin(rw, req)
	if user == nil {
		return
	}

	data := map[string]any{
		"Page":    wiki.NewStaticPage("Settings"),
		"Context": req.Context(),
		"Settings": map[string]any{
			"AllowAnonymousEdits":  a.RuntimeConfig.AllowAnonymousEditsGlobal,
			"MinimumPasswordLength": a.RuntimeConfig.MinimumPasswordLength,
			"CookieExpiry":          a.RuntimeConfig.CookieExpiry,
			"RenderWorkers":         a.RuntimeConfig.RenderWorkers,
		},
	}

	if msg := req.URL.Query().Get("msg"); msg != "" {
		data["calloutMessage"] = msg
		data["calloutClasses"] = "pw-success"
	}
	if errMsg := req.URL.Query().Get("err"); errMsg != "" {
		data["calloutMessage"] = errMsg
		data["calloutClasses"] = "pw-error"
	}

	err := a.RenderTemplate(rw, "settings.html", "index.html", data)
	check(err)
}

// ManageSettingsPostHandler handles POST requests to update runtime settings.
func (a *App) ManageSettingsPostHandler(rw http.ResponseWriter, req *http.Request) {
	user := a.RequireAdmin(rw, req)
	if user == nil {
		return
	}

	// Parse allow_anonymous_edits (checkbox: present = true, absent = false)
	allowAnon := req.PostFormValue("allow_anonymous_edits") == "on"

	// Parse and validate minimum_password_length
	minPwLen, err := strconv.Atoi(req.PostFormValue("minimum_password_length"))
	if err != nil || minPwLen < 1 {
		http.Redirect(rw, req, "/manage/settings?err=Minimum+password+length+must+be+at+least+1", http.StatusSeeOther)
		return
	}

	// Parse and validate cookie_expiry
	cookieExpiry, err := strconv.Atoi(req.PostFormValue("cookie_expiry"))
	if err != nil || cookieExpiry < 1 {
		http.Redirect(rw, req, "/manage/settings?err=Cookie+expiry+must+be+at+least+1+second", http.StatusSeeOther)
		return
	}

	// Parse and validate render_workers
	renderWorkers, err := strconv.Atoi(req.PostFormValue("render_workers"))
	if err != nil || renderWorkers < 0 {
		http.Redirect(rw, req, "/manage/settings?err=Render+workers+must+be+0+or+greater", http.StatusSeeOther)
		return
	}

	// Update each setting in the database
	updates := []struct {
		key   string
		value string
	}{
		{wiki.SettingAllowAnonymousEditsGlobal, strconv.FormatBool(allowAnon)},
		{wiki.SettingMinPasswordLength, strconv.Itoa(minPwLen)},
		{wiki.SettingCookieExpiry, strconv.Itoa(cookieExpiry)},
		{wiki.SettingRenderWorkers, strconv.Itoa(renderWorkers)},
	}

	for _, u := range updates {
		if err := wiki.UpdateSetting(a.DB, u.key, u.value); err != nil {
			slog.Error("failed to update setting", "key", u.key, "error", err)
			http.Redirect(rw, req, "/manage/settings?err="+fmt.Sprintf("Failed+to+update+%s", u.key), http.StatusSeeOther)
			return
		}
	}

	// Update in-memory config
	a.RuntimeConfig.AllowAnonymousEditsGlobal = allowAnon
	a.RuntimeConfig.MinimumPasswordLength = minPwLen
	a.RuntimeConfig.CookieExpiry = cookieExpiry
	a.RuntimeConfig.RenderWorkers = renderWorkers

	slog.Info("settings updated", "acting_user", user.ScreenName)
	http.Redirect(rw, req, "/manage/settings?msg=Settings+saved", http.StatusSeeOther)
}
