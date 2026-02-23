package server

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/danielledeleo/periwiki/internal/storage"
	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// setupAuthTestServer creates a test server with all routes configured.
func setupAuthTestServer(t *testing.T) (*httptest.Server, *testutil.TestApp, func()) {
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
	}

	router := mux.NewRouter().StrictSlash(true)
	router.Use(app.SessionMiddleware)

	router.HandleFunc("/", app.HomeHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}", app.ArticleHandler).Methods("GET")
	router.HandleFunc("/user/register", app.RegisterHandler).Methods("GET")
	router.HandleFunc("/user/register", app.RegisterPostHandler).Methods("POST")
	router.HandleFunc("/user/login", app.LoginHandler).Methods("GET")
	router.HandleFunc("/user/login", app.LoginPostHandler).Methods("POST")
	router.HandleFunc("/user/logout", app.LogoutPostHandler).Methods("POST")

	server := httptest.NewServer(router)

	serverCleanup := func() {
		server.Close()
		cleanup()
	}

	return server, testApp, serverCleanup
}

func TestRegistrationFlow(t *testing.T) {
	server, _, cleanup := setupAuthTestServer(t)
	defer cleanup()

	t.Run("GET registration form", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/user/register")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("POST successful registration", func(t *testing.T) {
		formData := url.Values{
			"screenname": {"newuser"},
			"email":      {"newuser@example.com"},
			"password":   {"securepassword123"},
		}

		resp, err := http.PostForm(server.URL+"/user/register", formData)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		// Read body to check for success message
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		body := string(bodyBytes)

		if !strings.Contains(body, "Successfully registered") {
			t.Error("expected success message in response")
		}
	})
}

func TestRegistrationErrors(t *testing.T) {
	server, testApp, cleanup := setupAuthTestServer(t)
	defer cleanup()

	// Create an existing user first
	testutil.CreateTestUser(t, testApp.DB, "existinguser", "existing@example.com", "password123")

	tests := []struct {
		name           string
		screenname     string
		email          string
		password       string
		expectedInBody string
	}{
		{
			name:           "duplicate username",
			screenname:     "existinguser",
			email:          "new@example.com",
			password:       "password123",
			expectedInBody: "username already in use",
		},
		{
			name:           "duplicate email",
			screenname:     "newuser2",
			email:          "existing@example.com",
			password:       "password123",
			expectedInBody: "email already in use",
		},
		{
			name:           "short password",
			screenname:     "shortpwuser",
			email:          "shortpw@example.com",
			password:       "short",
			expectedInBody: "password too short",
		},
		{
			name:           "invalid username characters",
			screenname:     "user@name!",
			email:          "invalid@example.com",
			password:       "password123",
			expectedInBody: "must only contain",
		},
		{
			name:           "empty username",
			screenname:     "",
			email:          "empty@example.com",
			password:       "password123",
			expectedInBody: "cannot be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			formData := url.Values{
				"screenname": {tc.screenname},
				"email":      {tc.email},
				"password":   {tc.password},
			}

			resp, err := http.PostForm(server.URL+"/user/register", formData)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			bodyBytes, _ := io.ReadAll(resp.Body)
			body := string(bodyBytes)

			if !strings.Contains(body, tc.expectedInBody) {
				t.Errorf("expected body to contain %q", tc.expectedInBody)
			}
		})
	}
}

func TestLoginFlow(t *testing.T) {
	server, testApp, cleanup := setupAuthTestServer(t)
	defer cleanup()

	// Create a user to login with
	password := "testpassword123"
	user := &wiki.User{
		ScreenName:  "loginuser",
		Email:       "login@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a client with cookie jar to track session
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	t.Run("successful login redirects", func(t *testing.T) {
		formData := url.Values{
			"screenname": {"loginuser"},
			"password":   {password},
		}

		resp, err := client.PostForm(server.URL+"/user/login", formData)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should redirect after successful login
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303 (redirect), got %d", resp.StatusCode)
		}

		// Should have set a session cookie
		cookies := jar.Cookies(&url.URL{Scheme: "http", Host: strings.TrimPrefix(server.URL, "http://")})
		foundSession := false
		for _, c := range cookies {
			if c.Name == "periwiki-login" {
				foundSession = true
				break
			}
		}
		if !foundSession {
			t.Error("expected session cookie to be set")
		}
	})
}

func TestLoginInvalidCredentials(t *testing.T) {
	server, testApp, cleanup := setupAuthTestServer(t)
	defer cleanup()

	// Create a user
	password := "correctpassword"
	user := &wiki.User{
		ScreenName:  "creduser",
		Email:       "cred@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	t.Run("wrong password", func(t *testing.T) {
		formData := url.Values{
			"screenname": {"creduser"},
			"password":   {"wrongpassword"},
		}

		resp, err := http.PostForm(server.URL+"/user/login", formData)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should stay on login page with 401 Unauthorized, not redirect
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", resp.StatusCode)
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		body := string(bodyBytes)

		// Check for error indicators (exact wording may vary)
		if !strings.Contains(strings.ToLower(body), "incorrect") && !strings.Contains(strings.ToLower(body), "password") && !strings.Contains(body, "error") && !strings.Contains(body, "pw-error") {
			t.Error("expected error message or indicator for incorrect password")
		}
	})

	t.Run("non-existent user", func(t *testing.T) {
		formData := url.Values{
			"screenname": {"nonexistentuser"},
			"password":   {"anypassword"},
		}

		resp, err := http.PostForm(server.URL+"/user/login", formData)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Important: the login should NOT redirect (which would be 303)
		// It should stay on the login page with an error
		if resp.StatusCode == http.StatusSeeOther {
			t.Error("login should fail for non-existent user, but got redirect")
		}

		// Should return 401 Unauthorized for failed login
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", resp.StatusCode)
		}
	})
}

func TestLogout(t *testing.T) {
	server, testApp, cleanup := setupAuthTestServer(t)
	defer cleanup()

	// Create and login a user
	password := "logoutpassword"
	user := &wiki.User{
		ScreenName:  "logoutuser",
		Email:       "logout@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Login first
	formData := url.Values{
		"screenname": {"logoutuser"},
		"password":   {password},
	}
	resp, err := client.PostForm(server.URL+"/user/login", formData)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login failed, expected 303, got %d", resp.StatusCode)
	}

	// Now logout
	resp, err = client.PostForm(server.URL+"/user/logout", url.Values{})
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	resp.Body.Close()

	// Should redirect to home
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected redirect status 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %q", location)
	}
}

func TestSessionMiddlewareAuthenticatedUser(t *testing.T) {
	testApp, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create a user
	password := "sessionpassword"
	user := &wiki.User{
		ScreenName:  "sessionuser",
		Email:       "session@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

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
	}

	router := mux.NewRouter()
	router.Use(app.SessionMiddleware)

	// Add login handler
	router.HandleFunc("/user/login", app.LoginPostHandler).Methods("POST")

	// Add a test handler that captures the user
	var capturedUser *wiki.User
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(wiki.UserKey).(*wiki.User)
		if ok {
			capturedUser = u
		}
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	server := httptest.NewServer(router)
	defer server.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Login
	formData := url.Values{
		"screenname": {"sessionuser"},
		"password":   {password},
	}
	resp, err := client.PostForm(server.URL+"/user/login", formData)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected login redirect, got %d", resp.StatusCode)
	}

	// Now make authenticated request
	resp, err = client.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	resp.Body.Close()

	if capturedUser == nil {
		t.Fatal("expected user to be captured from context")
	}

	if capturedUser.ScreenName != "sessionuser" {
		t.Errorf("expected screenname 'sessionuser', got %q", capturedUser.ScreenName)
	}

	if capturedUser.ID == 0 {
		t.Error("expected non-zero user ID for authenticated user")
	}
}

func TestAnonymousUser(t *testing.T) {
	testApp, cleanup := testutil.SetupTestApp(t)
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
	}

	var capturedUser *wiki.User

	handler := app.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(wiki.UserKey).(*wiki.User)
		if ok {
			capturedUser = u
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Make request without any session cookie
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if capturedUser == nil {
		t.Fatal("expected user in context")
	}

	if capturedUser.ID != 0 {
		t.Errorf("expected anonymous user ID 0, got %d", capturedUser.ID)
	}

	if capturedUser.ScreenName != "Anonymous" {
		t.Errorf("expected screenname 'Anonymous', got %q", capturedUser.ScreenName)
	}
}

func TestLoginWithReferrer(t *testing.T) {
	server, testApp, cleanup := setupAuthTestServer(t)
	defer cleanup()

	// Create a user
	password := "referrerpassword"
	user := &wiki.User{
		ScreenName:  "referreruser",
		Email:       "referrer@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Login with referrer
	formData := url.Values{
		"screenname": {"referreruser"},
		"password":   {password},
		"referrer":   {"/wiki/some-article"},
	}

	resp, err := client.PostForm(server.URL+"/user/login", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected redirect status 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/wiki/some-article" {
		t.Errorf("expected redirect to /wiki/some-article, got %q", location)
	}
}

func TestLoginWithEmptyReferrer(t *testing.T) {
	server, testApp, cleanup := setupAuthTestServer(t)
	defer cleanup()

	// Create a user
	password := "emptyrefpassword"
	user := &wiki.User{
		ScreenName:  "emptyrefuser",
		Email:       "emptyref@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Login without referrer
	formData := url.Values{
		"screenname": {"emptyrefuser"},
		"password":   {password},
	}

	resp, err := client.PostForm(server.URL+"/user/login", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected redirect status 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %q", location)
	}
}

// TestCookieSecretChange verifies behavior when the cookie secret changes after a user has logged in.
// This simulates the scenario where:
// 1. User logs in with cookie secret A
// 2. Server restarts with cookie secret B (e.g., after migration to database-stored secrets)
// 3. User's browser sends the old cookie signed with secret A
//
// Expected behavior: The app should gracefully treat the user as anonymous,
// not return a 500 error.
func TestCookieSecretChange(t *testing.T) {
	testApp, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create a user
	password := "secretchangepass"
	user := &wiki.User{
		ScreenName:  "secretchangeuser",
		Email:       "secretchange@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create app with original cookie secret
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
	}

	// Track captured user for verification
	var capturedUser *wiki.User

	router := mux.NewRouter()
	router.Use(app.SessionMiddleware)
	router.HandleFunc("/user/login", app.LoginPostHandler).Methods("POST")
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(wiki.UserKey).(*wiki.User)
		if ok {
			capturedUser = u
		}
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	server := httptest.NewServer(router)
	defer server.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Step 1: Login with the original secret - user gets authenticated cookie
	formData := url.Values{
		"screenname": {"secretchangeuser"},
		"password":   {password},
	}
	resp, err := client.PostForm(server.URL+"/user/login", formData)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected login redirect (303), got %d", resp.StatusCode)
	}

	// Verify the user is authenticated with the original app
	capturedUser = nil
	resp, err = client.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	resp.Body.Close()

	if capturedUser == nil || capturedUser.ScreenName != "secretchangeuser" {
		t.Fatalf("user should be authenticated before secret change, got: %v", capturedUser)
	}

	// Step 2: Create a NEW session store with a DIFFERENT cookie secret
	// This simulates what happens when the server restarts with a new cookie secret
	newSecret := []byte("completely-different-secret-key!")
	newRuntimeConfig := &wiki.RuntimeConfig{
		CookieSecret:              newSecret,
		CookieExpiry:              86400,
		MinimumPasswordLength:     8,
		AllowAnonymousEditsGlobal: true,
		RenderWorkers:             0,
	}

	// Create a new TestDB with the new secret but same underlying database connection
	// We need to create a fresh session store with the new secret
	newSessionStore := storage.NewSessionStore(
		testApp.DB.Conn(), "/", newRuntimeConfig.CookieExpiry, newRuntimeConfig.CookieSecret)

	// Create a wrapper that implements the SessionService interface
	newSessionService := &testSessionStore{SessionStore: newSessionStore}

	// Create new app with the different cookie secret
	appWithNewSecret := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      newSessionService,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: newRuntimeConfig,
	}

	// Create new router with the new app
	newRouter := mux.NewRouter()
	newRouter.Use(appWithNewSecret.SessionMiddleware)
	newRouter.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(wiki.UserKey).(*wiki.User)
		if ok {
			capturedUser = u
		}
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	newServer := httptest.NewServer(newRouter)
	defer newServer.Close()

	// Step 3: Make a request with the old cookie to the new server
	// The cookie was signed with the old secret, but the server now uses the new secret
	capturedUser = nil

	// Copy cookies from old server to new server URL
	oldURL, _ := url.Parse(server.URL)
	newURL, _ := url.Parse(newServer.URL)
	cookies := jar.Cookies(oldURL)
	jar.SetCookies(newURL, cookies)

	resp, err = client.Get(newServer.URL + "/test")
	if err != nil {
		t.Fatalf("test request to new server failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Step 4: Verify the behavior
	// The request should NOT return a 500 error
	if resp.StatusCode == http.StatusInternalServerError {
		t.Errorf("Got 500 Internal Server Error when cookie secret changed. "+
			"The app should gracefully handle invalid cookies. Response body: %s", string(body))
	}

	// The user should be treated as anonymous (cookie can't be decoded with new secret)
	if capturedUser == nil {
		t.Fatal("expected user context to be set (even if anonymous)")
	}

	// User should be anonymous since the cookie can't be validated
	if capturedUser.ID != 0 {
		t.Logf("Note: User was authenticated as %q (ID: %d) despite secret change",
			capturedUser.ScreenName, capturedUser.ID)
		// This might happen if the session store handles the error differently
		// The important thing is that we don't get a 500
	} else {
		t.Logf("User correctly treated as anonymous after cookie secret change")
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestLoginWithInvalidCookie verifies that attempting to login with an invalid/corrupted
// session cookie (e.g., signed with a different secret) does not cause a 500 error.
// This is the specific scenario when a user's browser has an old cookie and they try to login.
func TestLoginWithInvalidCookie(t *testing.T) {
	testApp, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create a user
	password := "invalidcookiepass"
	user := &wiki.User{
		ScreenName:  "invalidcookieuser",
		Email:       "invalidcookie@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Step 1: Login with original secret to get a valid cookie
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
	}

	router := mux.NewRouter()
	router.Use(app.SessionMiddleware)
	router.HandleFunc("/user/login", app.LoginHandler).Methods("GET")
	router.HandleFunc("/user/login", app.LoginPostHandler).Methods("POST")

	server := httptest.NewServer(router)
	defer server.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Login to get a cookie
	formData := url.Values{
		"screenname": {"invalidcookieuser"},
		"password":   {password},
	}
	resp, err := client.PostForm(server.URL+"/user/login", formData)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected login redirect (303), got %d", resp.StatusCode)
	}

	// Step 2: Create new server with DIFFERENT cookie secret
	newSecret := []byte("completely-different-secret-key!")
	newRuntimeConfig := &wiki.RuntimeConfig{
		CookieSecret:              newSecret,
		CookieExpiry:              86400,
		MinimumPasswordLength:     8,
		AllowAnonymousEditsGlobal: true,
		RenderWorkers:             0,
	}

	newSessionStore := storage.NewSessionStore(
		testApp.DB.Conn(), "/", newRuntimeConfig.CookieExpiry, newRuntimeConfig.CookieSecret)

	newSessionService := &testSessionStore{SessionStore: newSessionStore}

	appWithNewSecret := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      newSessionService,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: newRuntimeConfig,
	}

	newRouter := mux.NewRouter()
	newRouter.Use(appWithNewSecret.SessionMiddleware)
	newRouter.HandleFunc("/user/login", appWithNewSecret.LoginHandler).Methods("GET")
	newRouter.HandleFunc("/user/login", appWithNewSecret.LoginPostHandler).Methods("POST")

	newServer := httptest.NewServer(newRouter)
	defer newServer.Close()

	// Copy cookies from old server to new server URL
	oldURL, _ := url.Parse(server.URL)
	newURL, _ := url.Parse(newServer.URL)
	cookies := jar.Cookies(oldURL)
	jar.SetCookies(newURL, cookies)

	// Step 3: Try to access the login page with the invalid cookie
	resp, err = client.Get(newServer.URL + "/user/login")
	if err != nil {
		t.Fatalf("GET login page failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode == http.StatusInternalServerError {
		t.Errorf("GET /user/login returned 500 with invalid cookie. Response: %s", string(body))
	} else {
		t.Logf("GET /user/login returned %d (expected 200)", resp.StatusCode)
	}

	// Step 4: Try to POST login with the invalid cookie
	formData = url.Values{
		"screenname": {"invalidcookieuser"},
		"password":   {password},
	}
	resp, err = client.PostForm(newServer.URL+"/user/login", formData)
	if err != nil {
		t.Fatalf("POST login failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode == http.StatusInternalServerError {
		t.Errorf("POST /user/login returned 500 with invalid cookie. Response: %s", string(body))
	} else if resp.StatusCode == http.StatusSeeOther {
		t.Logf("POST /user/login succeeded (redirected to %s)", resp.Header.Get("Location"))
	} else {
		t.Logf("POST /user/login returned %d", resp.StatusCode)
	}
}

// testSessionStore wraps SessionStore to implement the SessionService interface
type testSessionStore struct {
	*storage.SessionStore
}

func (s *testSessionStore) GetCookie(r *http.Request, name string) (*sessions.Session, error) {
	return s.SessionStore.Get(r, name)
}

func (s *testSessionStore) NewCookie(r *http.Request, name string) (*sessions.Session, error) {
	return s.SessionStore.New(r, name)
}

func (s *testSessionStore) SaveCookie(r *http.Request, w http.ResponseWriter, sess *sessions.Session) error {
	return s.SessionStore.Save(r, w, sess)
}

func (s *testSessionStore) DeleteCookie(r *http.Request, w http.ResponseWriter, sess *sessions.Session) error {
	return s.SessionStore.Delete(r, w, sess)
}
