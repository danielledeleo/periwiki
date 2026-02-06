package wiki

import "testing"

func TestUserIsAdmin(t *testing.T) {
	tests := []struct {
		name     string
		user     *User
		expected bool
	}{
		{
			name:     "admin user",
			user:     &User{ID: 1, Role: RoleAdmin},
			expected: true,
		},
		{
			name:     "regular user",
			user:     &User{ID: 2, Role: RoleUser},
			expected: false,
		},
		{
			name:     "anonymous user (no role)",
			user:     &User{ID: 0, Role: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.IsAdmin(); got != tt.expected {
				t.Errorf("User.IsAdmin() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestUserIsAnonymous(t *testing.T) {
	tests := []struct {
		name     string
		user     *User
		expected bool
	}{
		{
			name:     "anonymous user (ID=0)",
			user:     &User{ID: 0},
			expected: true,
		},
		{
			name:     "authenticated user (ID=1)",
			user:     &User{ID: 1},
			expected: false,
		},
		{
			name:     "authenticated user (ID=100)",
			user:     &User{ID: 100},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.IsAnonymous(); got != tt.expected {
				t.Errorf("User.IsAnonymous() = %v, want %v", got, tt.expected)
			}
		})
	}
}
