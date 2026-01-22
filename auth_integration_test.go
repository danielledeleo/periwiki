package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/gorilla/mux"
)

// setupAuthTestServer creates a test server with all routes configured.
func setupAuthTestServer(t *testing.T) (*httptest.Server, *testutil.TestApp, func()) {
	t.Helper()

	testApp, cleanup := testutil.SetupTestApp(t)

	app := &app{
		Templater:    testApp.Templater,
		WikiModel:    testApp.WikiModel,
		specialPages: testApp.SpecialPages,
	}

	router := mux.NewRouter().StrictSlash(true)
	router.Use(app.SessionMiddleware)

	router.HandleFunc("/", app.homeHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}", app.articleHandler).Methods("GET")
	router.HandleFunc("/user/register", app.registerHandler).Methods("GET")
	router.HandleFunc("/user/register", app.registerPostHandler).Methods("POST")
	router.HandleFunc("/user/login", app.loginHander).Methods("GET")
	router.HandleFunc("/user/login", app.loginPostHander).Methods("POST")
	router.HandleFunc("/user/logout", app.logoutPostHander).Methods("POST")

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
	err := testApp.PostUser(user)
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
	err := testApp.PostUser(user)
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
	err := testApp.PostUser(user)
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
	err := testApp.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	app := &app{
		Templater:    testApp.Templater,
		WikiModel:    testApp.WikiModel,
		specialPages: testApp.SpecialPages,
	}

	router := mux.NewRouter()
	router.Use(app.SessionMiddleware)

	// Add login handler
	router.HandleFunc("/user/login", app.loginPostHander).Methods("POST")

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

	app := &app{
		Templater:    testApp.Templater,
		WikiModel:    testApp.WikiModel,
		specialPages: testApp.SpecialPages,
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
	err := testApp.PostUser(user)
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
	err := testApp.PostUser(user)
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
