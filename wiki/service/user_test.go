package service_test

import (
	"testing"

	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
)

func TestPostUser(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	t.Run("creates valid user", func(t *testing.T) {
		user := &wiki.User{
			ScreenName:  "validuser",
			Email:       "valid@example.com",
			RawPassword: "securepassword123",
		}

		err := app.Users.PostUser(user)
		if err != nil {
			t.Fatalf("PostUser failed: %v", err)
		}

		// Verify user was created
		retrieved, err := app.Users.GetUserByScreenName("validuser")
		if err != nil {
			t.Fatalf("GetUserByScreenName failed: %v", err)
		}

		if retrieved.ScreenName != "validuser" {
			t.Errorf("expected screenname 'validuser', got %q", retrieved.ScreenName)
		}
		if retrieved.Email != "valid@example.com" {
			t.Errorf("expected email 'valid@example.com', got %q", retrieved.Email)
		}
	})

	t.Run("allows unicode usernames", func(t *testing.T) {
		user := &wiki.User{
			ScreenName:  "用户名", // Chinese characters
			Email:       "unicode@example.com",
			RawPassword: "securepassword123",
		}

		err := app.Users.PostUser(user)
		if err != nil {
			t.Fatalf("PostUser with unicode failed: %v", err)
		}
	})

	t.Run("allows hyphens and underscores", func(t *testing.T) {
		user := &wiki.User{
			ScreenName:  "user-name_123",
			Email:       "special@example.com",
			RawPassword: "securepassword123",
		}

		err := app.Users.PostUser(user)
		if err != nil {
			t.Fatalf("PostUser with special chars failed: %v", err)
		}
	})
}

func TestPostUserValidation(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	tests := []struct {
		name        string
		screenname  string
		email       string
		password    string
		expectedErr error
	}{
		{
			name:        "empty username",
			screenname:  "",
			email:       "test@example.com",
			password:    "password123",
			expectedErr: wiki.ErrEmptyUsername,
		},
		{
			name:        "short password",
			screenname:  "testuser",
			email:       "test@example.com",
			password:    "short",
			expectedErr: nil, // will contain ErrPasswordTooShort but wrapped
		},
		{
			name:        "invalid characters",
			screenname:  "user@name",
			email:       "test@example.com",
			password:    "password123",
			expectedErr: wiki.ErrBadUsername,
		},
		{
			name:        "invalid characters with spaces",
			screenname:  "user name",
			email:       "test@example.com",
			password:    "password123",
			expectedErr: wiki.ErrBadUsername,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user := &wiki.User{
				ScreenName:  tc.screenname,
				Email:       tc.email,
				RawPassword: tc.password,
			}

			err := app.Users.PostUser(user)

			if tc.expectedErr != nil {
				if err != tc.expectedErr {
					t.Errorf("expected error %v, got: %v", tc.expectedErr, err)
				}
			} else {
				// For short password, we just check that an error occurred
				if tc.name == "short password" && err == nil {
					t.Error("expected error for short password")
				}
			}
		})
	}
}

func TestCheckUserPassword(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create a user with known password
	password := "correctpassword123"
	user := &wiki.User{
		ScreenName:  "passwordtest",
		Email:       "password@example.com",
		RawPassword: password,
	}
	err := app.Users.PostUser(user)
	if err != nil {
		t.Fatalf("PostUser failed: %v", err)
	}

	t.Run("correct password", func(t *testing.T) {
		checkUser := &wiki.User{
			ScreenName:  "passwordtest",
			RawPassword: password,
		}
		err := app.Users.CheckUserPassword(checkUser)
		if err != nil {
			t.Errorf("expected no error for correct password, got: %v", err)
		}
	})

	t.Run("incorrect password", func(t *testing.T) {
		checkUser := &wiki.User{
			ScreenName:  "passwordtest",
			RawPassword: "wrongpassword",
		}
		err := app.Users.CheckUserPassword(checkUser)
		if err != wiki.ErrIncorrectPassword {
			t.Errorf("expected ErrIncorrectPassword, got: %v", err)
		}
	})

	t.Run("non-existent user", func(t *testing.T) {
		checkUser := &wiki.User{
			ScreenName:  "nonexistent",
			RawPassword: "anypassword",
		}
		err := app.Users.CheckUserPassword(checkUser)
		if err != wiki.ErrUsernameNotFound {
			t.Errorf("expected ErrUsernameNotFound, got: %v", err)
		}
	})
}
