package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/gorilla/mux"
)

func setupManageTestServer(t *testing.T) (*httptest.Server, *testutil.TestApp, func()) {
	t.Helper()

	testApp, cleanup := testutil.SetupTestApp(t)

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
		DB:            testApp.RawDB,
	}

	router := mux.NewRouter().StrictSlash(true)
	router.Use(app.SessionMiddleware)

	router.HandleFunc("/user/login", app.LoginHandler).Methods("GET")
	router.HandleFunc("/user/login", app.LoginPostHandler).Methods("POST")
	router.HandleFunc("/manage/users", app.ManageUsersHandler).Methods("GET")
	router.HandleFunc("/manage/users/{id:[0-9]+}", app.ManageUserRoleHandler).Methods("POST")
	router.HandleFunc("/manage/settings", app.ManageSettingsHandler).Methods("GET")
	router.HandleFunc("/manage/settings", app.ManageSettingsPostHandler).Methods("POST")
	router.HandleFunc("/manage/settings/reset-main-page", app.ResetMainPageHandler).Methods("POST")
	router.HandleFunc("/manage/content", app.ManageContentHandler).Methods("GET")

	server := httptest.NewServer(router)

	serverCleanup := func() {
		server.Close()
		cleanup()
	}

	return server, testApp, serverCleanup
}

func TestManageUsersRequiresAdmin(t *testing.T) {
	server, testApp, cleanup := setupManageTestServer(t)
	defer cleanup()

	t.Run("anonymous user redirected to login", func(t *testing.T) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Get(server.URL + "/manage/users")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected 303 redirect, got %d", resp.StatusCode)
		}
		location := resp.Header.Get("Location")
		if !strings.Contains(location, "/user/login") {
			t.Errorf("expected redirect to login, got %q", location)
		}
	})

	t.Run("non-admin user gets 403", func(t *testing.T) {
		user := testutil.CreateTestUser(t, testApp.DB, "regular", "regular@example.com", "password1234")

		req := testutil.MakeTestRequest("GET", "/manage/users", user)
		rw := httptest.NewRecorder()

		app := &App{
			Templater:     testApp.Templater,
			Articles:      testApp.Articles,
			Users:         testApp.Users,
			Sessions:      testApp.Sessions,
			Rendering:     testApp.Rendering,
			Preferences:   testApp.Preferences,
			SpecialPages:  testApp.SpecialPages,
			Config:        testApp.Config,
			RuntimeConfig: testApp.RuntimeConfig,
			DB:            testApp.RawDB,
		}

		app.ManageUsersHandler(rw, req)

		if rw.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rw.Code)
		}
	})

	t.Run("admin user gets 200", func(t *testing.T) {
		admin := testutil.CreateTestAdmin(t, testApp.DB, "admin", "admin@example.com", "password1234")

		req := testutil.MakeTestRequest("GET", "/manage/users", admin)
		rw := httptest.NewRecorder()

		app := &App{
			Templater:     testApp.Templater,
			Articles:      testApp.Articles,
			Users:         testApp.Users,
			Sessions:      testApp.Sessions,
			Rendering:     testApp.Rendering,
			Preferences:   testApp.Preferences,
			SpecialPages:  testApp.SpecialPages,
			Config:        testApp.Config,
			RuntimeConfig: testApp.RuntimeConfig,
			DB:            testApp.RawDB,
		}

		app.ManageUsersHandler(rw, req)

		if rw.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rw.Code)
		}
	})
}

func TestManageUserRoleChange(t *testing.T) {
	_, testApp, cleanup := setupManageTestServer(t)
	defer cleanup()

	admin := testutil.CreateTestAdmin(t, testApp.DB, "admin", "admin@example.com", "password1234")
	target := testutil.CreateTestUser(t, testApp.DB, "target", "target@example.com", "password1234")

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
		DB:            testApp.RawDB,
	}

	t.Run("admin can change role via POST", func(t *testing.T) {
		form := url.Values{"role": {"admin"}}
		req := httptest.NewRequest("POST", "/manage/users/"+strings.TrimSpace(fmt.Sprintf("%d", target.ID)), strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = testutil.RequestWithUser(req, admin)

		// Set up mux vars
		router := mux.NewRouter()
		router.HandleFunc("/manage/users/{id:[0-9]+}", app.ManageUserRoleHandler).Methods("POST")

		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, req)

		if rw.Code != http.StatusSeeOther {
			t.Errorf("expected 303 redirect, got %d", rw.Code)
		}

		// Verify role persisted
		updated, err := testApp.Users.GetUserByID(target.ID)
		if err != nil {
			t.Fatalf("GetUserByID failed: %v", err)
		}
		if updated.Role != wiki.RoleAdmin {
			t.Errorf("expected admin role, got %q", updated.Role)
		}
	})

	t.Run("non-admin POST gets 403", func(t *testing.T) {
		nonAdmin := testutil.CreateTestUser(t, testApp.DB, "nonadmin", "nonadmin@example.com", "password1234")

		form := url.Values{"role": {"admin"}}
		req := httptest.NewRequest("POST", "/manage/users/"+fmt.Sprintf("%d", target.ID), strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = testutil.RequestWithUser(req, nonAdmin)

		router := mux.NewRouter()
		router.HandleFunc("/manage/users/{id:[0-9]+}", app.ManageUserRoleHandler).Methods("POST")

		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, req)

		if rw.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rw.Code)
		}
	})
}

func TestManageSettingsRequiresAdmin(t *testing.T) {
	_, testApp, cleanup := setupManageTestServer(t)
	defer cleanup()

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
		DB:            testApp.RawDB,
	}

	t.Run("non-admin user gets 403", func(t *testing.T) {
		user := testutil.CreateTestUser(t, testApp.DB, "regular", "regular@example.com", "password1234")

		req := testutil.MakeTestRequest("GET", "/manage/settings", user)
		rw := httptest.NewRecorder()

		app.ManageSettingsHandler(rw, req)

		if rw.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rw.Code)
		}
	})

	t.Run("admin user gets 200", func(t *testing.T) {
		admin := testutil.CreateTestAdmin(t, testApp.DB, "admin", "admin@example.com", "password1234")

		req := testutil.MakeTestRequest("GET", "/manage/settings", admin)
		rw := httptest.NewRecorder()

		app.ManageSettingsHandler(rw, req)

		if rw.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rw.Code)
		}
	})
}

func TestManageSettingsUpdate(t *testing.T) {
	_, testApp, cleanup := setupManageTestServer(t)
	defer cleanup()

	admin := testutil.CreateTestAdmin(t, testApp.DB, "admin", "admin@example.com", "password1234")

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
		DB:            testApp.RawDB,
	}

	t.Run("admin can update settings", func(t *testing.T) {
		form := url.Values{
			"allow_anonymous_edits":    {"on"},
			"minimum_password_length":  {"12"},
			"cookie_expiry":            {"3600"},
			"render_workers":           {"2"},
		}
		req := httptest.NewRequest("POST", "/manage/settings", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = testutil.RequestWithUser(req, admin)

		rw := httptest.NewRecorder()
		app.ManageSettingsPostHandler(rw, req)

		if rw.Code != http.StatusSeeOther {
			t.Errorf("expected 303 redirect, got %d", rw.Code)
		}
		location := rw.Header().Get("Location")
		if !strings.Contains(location, "msg=Settings") {
			t.Errorf("expected success redirect, got %q", location)
		}

		// Verify in-memory config was updated
		if app.RuntimeConfig.MinimumPasswordLength != 12 {
			t.Errorf("expected MinimumPasswordLength 12, got %d", app.RuntimeConfig.MinimumPasswordLength)
		}
		if app.RuntimeConfig.CookieExpiry != 3600 {
			t.Errorf("expected CookieExpiry 3600, got %d", app.RuntimeConfig.CookieExpiry)
		}
		if app.RuntimeConfig.RenderWorkers != 2 {
			t.Errorf("expected RenderWorkers 2, got %d", app.RuntimeConfig.RenderWorkers)
		}
		if !app.RuntimeConfig.AllowAnonymousEditsGlobal {
			t.Error("expected AllowAnonymousEditsGlobal true")
		}
	})

	t.Run("unchecked checkbox disables anonymous edits", func(t *testing.T) {
		form := url.Values{
			// No "allow_anonymous_edits" key = unchecked
			"minimum_password_length":  {"8"},
			"cookie_expiry":            {"86400"},
			"render_workers":           {"0"},
		}
		req := httptest.NewRequest("POST", "/manage/settings", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = testutil.RequestWithUser(req, admin)

		rw := httptest.NewRecorder()
		app.ManageSettingsPostHandler(rw, req)

		if rw.Code != http.StatusSeeOther {
			t.Errorf("expected 303 redirect, got %d", rw.Code)
		}

		if app.RuntimeConfig.AllowAnonymousEditsGlobal {
			t.Error("expected AllowAnonymousEditsGlobal false when checkbox unchecked")
		}
	})

	t.Run("invalid password length rejected", func(t *testing.T) {
		form := url.Values{
			"minimum_password_length":  {"0"},
			"cookie_expiry":            {"86400"},
			"render_workers":           {"0"},
		}
		req := httptest.NewRequest("POST", "/manage/settings", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = testutil.RequestWithUser(req, admin)

		rw := httptest.NewRecorder()
		app.ManageSettingsPostHandler(rw, req)

		if rw.Code != http.StatusSeeOther {
			t.Errorf("expected 303 redirect, got %d", rw.Code)
		}
		location := rw.Header().Get("Location")
		if !strings.Contains(location, "err=") {
			t.Errorf("expected error redirect, got %q", location)
		}
	})

	t.Run("negative render workers rejected", func(t *testing.T) {
		form := url.Values{
			"minimum_password_length":  {"8"},
			"cookie_expiry":            {"86400"},
			"render_workers":           {"-1"},
		}
		req := httptest.NewRequest("POST", "/manage/settings", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = testutil.RequestWithUser(req, admin)

		rw := httptest.NewRecorder()
		app.ManageSettingsPostHandler(rw, req)

		if rw.Code != http.StatusSeeOther {
			t.Errorf("expected 303 redirect, got %d", rw.Code)
		}
		location := rw.Header().Get("Location")
		if !strings.Contains(location, "err=") {
			t.Errorf("expected error redirect, got %q", location)
		}
	})

	t.Run("non-admin POST gets 403", func(t *testing.T) {
		nonAdmin := testutil.CreateTestUser(t, testApp.DB, "nonadmin", "nonadmin@example.com", "password1234")

		form := url.Values{
			"minimum_password_length":  {"8"},
			"cookie_expiry":            {"86400"},
			"render_workers":           {"0"},
		}
		req := httptest.NewRequest("POST", "/manage/settings", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = testutil.RequestWithUser(req, nonAdmin)

		rw := httptest.NewRecorder()
		app.ManageSettingsPostHandler(rw, req)

		if rw.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rw.Code)
		}
	})
}

func TestManageContentRequiresAdmin(t *testing.T) {
	_, testApp, cleanup := setupManageTestServer(t)
	defer cleanup()

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
		DB:            testApp.RawDB,
		ContentInfo: &ContentInfo{
			Files: []ContentFileEntry{
				{Path: "templates/layouts/index.html", Source: "embedded"},
				{Path: "static/main.css", Source: "disk"},
			},
			BuildCommit: "abc123def456",
			SourceURL:   "https://github.com/example/repo/blob/abc123def456",
		},
	}

	t.Run("non-admin user gets 403", func(t *testing.T) {
		user := testutil.CreateTestUser(t, testApp.DB, "regular2", "regular2@example.com", "password1234")

		req := testutil.MakeTestRequest("GET", "/manage/content", user)
		rw := httptest.NewRecorder()

		app.ManageContentHandler(rw, req)

		if rw.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rw.Code)
		}
	})

	t.Run("admin user gets 200 with content tree", func(t *testing.T) {
		admin := testutil.CreateTestAdmin(t, testApp.DB, "admin2", "admin2@example.com", "password1234")

		req := testutil.MakeTestRequest("GET", "/manage/content", admin)
		rw := httptest.NewRecorder()

		app.ManageContentHandler(rw, req)

		if rw.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rw.Code)
		}

		body := rw.Body.String()

		// Check for commit info
		if !strings.Contains(body, "abc123def456") {
			t.Error("expected commit hash in response body")
		}

		// Check for file count
		if !strings.Contains(body, "2 files") {
			t.Error("expected file count in response body")
		}

		// Check for override count
		if !strings.Contains(body, "1 overridden") {
			t.Error("expected override count in response body")
		}

		// Check for source badges
		if !strings.Contains(body, "embedded") {
			t.Error("expected 'embedded' source badge")
		}
		if !strings.Contains(body, "disk") {
			t.Error("expected 'disk' source badge")
		}
	})

	t.Run("nil content info renders gracefully", func(t *testing.T) {
		admin := testutil.CreateTestAdmin(t, testApp.DB, "admin3", "admin3@example.com", "password1234")

		appNil := &App{
			Templater:     testApp.Templater,
			Articles:      testApp.Articles,
			Users:         testApp.Users,
			Sessions:      testApp.Sessions,
			Rendering:     testApp.Rendering,
			Preferences:   testApp.Preferences,
			SpecialPages:  testApp.SpecialPages,
			Config:        testApp.Config,
			RuntimeConfig: testApp.RuntimeConfig,
			DB:            testApp.RawDB,
			ContentInfo:   nil,
		}

		req := testutil.MakeTestRequest("GET", "/manage/content", admin)
		rw := httptest.NewRecorder()

		appNil.ManageContentHandler(rw, req)

		if rw.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rw.Code)
		}
	})
}

func TestResetMainPage(t *testing.T) {
	_, testApp, cleanup := setupManageTestServer(t)
	defer cleanup()

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
		DB:            testApp.RawDB,
	}

	t.Run("non-admin user gets 403", func(t *testing.T) {
		user := testutil.CreateTestUser(t, testApp.DB, "resetnonadmin", "resetnonadmin@example.com", "password1234")

		req := testutil.MakeTestRequest("POST", "/manage/settings/reset-main-page", user)
		rw := httptest.NewRecorder()

		app.ResetMainPageHandler(rw, req)

		if rw.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rw.Code)
		}
	})

	t.Run("admin resets main page", func(t *testing.T) {
		admin := testutil.CreateTestAdmin(t, testApp.DB, "resetadmin", "resetadmin@example.com", "password1234")

		// Create Main_Page with custom content
		testutil.CreateTestArticle(t, testApp, "Main_Page", "Custom content", admin)

		req := testutil.MakeTestRequest("POST", "/manage/settings/reset-main-page", admin)
		rw := httptest.NewRecorder()

		app.ResetMainPageHandler(rw, req)

		if rw.Code != http.StatusSeeOther {
			t.Errorf("expected 303 redirect, got %d", rw.Code)
		}

		location := rw.Header().Get("Location")
		if !strings.Contains(location, "msg=") {
			t.Errorf("expected success message in redirect, got %q", location)
		}

		// Verify Main_Page was reset to default content
		article, err := testApp.Articles.GetArticle("Main_Page")
		if err != nil {
			t.Fatalf("failed to get Main_Page: %v", err)
		}
		if !strings.Contains(article.Markdown, "layout: mainpage") {
			t.Error("expected reset article to contain default frontmatter")
		}
	})
}

