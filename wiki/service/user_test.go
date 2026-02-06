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

func TestPostUserFirstUserPromotion(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := &wiki.User{
		ScreenName:  "firstuser",
		Email:       "first@example.com",
		RawPassword: "securepassword123",
	}

	err := app.Users.PostUser(user)
	if err != nil {
		t.Fatalf("PostUser failed: %v", err)
	}

	if user.ID != 1 {
		t.Fatalf("expected first user to have ID 1, got %d", user.ID)
	}

	if user.Role != wiki.RoleAdmin {
		t.Errorf("expected first user to be promoted to admin, got role %q", user.Role)
	}

	// Verify it persisted
	retrieved, err := app.Users.GetUserByScreenName("firstuser")
	if err != nil {
		t.Fatalf("GetUserByScreenName failed: %v", err)
	}
	if retrieved.Role != wiki.RoleAdmin {
		t.Errorf("expected retrieved user to have admin role, got %q", retrieved.Role)
	}
}

func TestPostUserSecondUserNotPromoted(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create first user
	first := &wiki.User{
		ScreenName:  "firstuser",
		Email:       "first@example.com",
		RawPassword: "securepassword123",
	}
	if err := app.Users.PostUser(first); err != nil {
		t.Fatalf("PostUser failed for first user: %v", err)
	}

	// Create second user
	second := &wiki.User{
		ScreenName:  "seconduser",
		Email:       "second@example.com",
		RawPassword: "securepassword123",
	}
	if err := app.Users.PostUser(second); err != nil {
		t.Fatalf("PostUser failed for second user: %v", err)
	}

	if second.Role == wiki.RoleAdmin {
		t.Errorf("expected second user to not be promoted, got role %q", second.Role)
	}
}

func TestGetAllUsers(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	testutil.CreateTestUser(t, app.DB, "alice", "alice@example.com", "password1234")
	testutil.CreateTestUser(t, app.DB, "bob", "bob@example.com", "password1234")

	users, err := app.Users.GetAllUsers()
	if err != nil {
		t.Fatalf("GetAllUsers failed: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	if users[0].ScreenName != "alice" {
		t.Errorf("expected first user to be alice, got %q", users[0].ScreenName)
	}
	if users[1].ScreenName != "bob" {
		t.Errorf("expected second user to be bob, got %q", users[1].ScreenName)
	}
}

func TestSetUserRole(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	admin := testutil.CreateTestAdmin(t, app.DB, "admin", "admin@example.com", "password1234")
	target := testutil.CreateTestUser(t, app.DB, "regular", "regular@example.com", "password1234")

	t.Run("admin can promote user", func(t *testing.T) {
		err := app.Users.SetUserRole(admin, target.ID, wiki.RoleAdmin)
		if err != nil {
			t.Fatalf("SetUserRole failed: %v", err)
		}
		updated, _ := app.Users.GetUserByID(target.ID)
		if updated.Role != wiki.RoleAdmin {
			t.Errorf("expected admin role, got %q", updated.Role)
		}
	})

	t.Run("admin can demote other user", func(t *testing.T) {
		err := app.Users.SetUserRole(admin, target.ID, wiki.RoleUser)
		if err != nil {
			t.Fatalf("SetUserRole failed: %v", err)
		}
		updated, _ := app.Users.GetUserByID(target.ID)
		if updated.Role != wiki.RoleUser {
			t.Errorf("expected user role, got %q", updated.Role)
		}
	})

	t.Run("admin cannot demote self", func(t *testing.T) {
		err := app.Users.SetUserRole(admin, admin.ID, wiki.RoleUser)
		if err != wiki.ErrForbidden {
			t.Errorf("expected ErrForbidden, got: %v", err)
		}
	})

	t.Run("non-admin cannot change roles", func(t *testing.T) {
		nonAdmin := testutil.CreateTestUser(t, app.DB, "nonadmin", "nonadmin@example.com", "password1234")
		err := app.Users.SetUserRole(nonAdmin, target.ID, wiki.RoleAdmin)
		if err != wiki.ErrAdminRequired {
			t.Errorf("expected ErrAdminRequired, got: %v", err)
		}
	})

	t.Run("invalid role rejected", func(t *testing.T) {
		err := app.Users.SetUserRole(admin, target.ID, "superadmin")
		if err == nil {
			t.Error("expected error for invalid role")
		}
	})
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
