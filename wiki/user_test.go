package wiki

import "testing"

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
