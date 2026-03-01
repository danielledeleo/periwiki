package wiki

import "testing"

func TestIsTalkPage(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"Talk:Foo", true},
		{"Talk:Some_Article", true},
		{"Talk:", true},
		{"Foo", false},
		{"", false},
		{"TalkPage", false},
		{"talk:Foo", false}, // case-sensitive
	}

	for _, tt := range tests {
		if got := IsTalkPage(tt.url); got != tt.want {
			t.Errorf("IsTalkPage(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestTalkPageURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"Foo", "Talk:Foo"},
		{"Some_Article", "Talk:Some_Article"},
		{"", "Talk:"},
	}

	for _, tt := range tests {
		if got := TalkPageURL(tt.url); got != tt.want {
			t.Errorf("TalkPageURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestSubjectPageURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"Talk:Foo", "Foo"},
		{"Talk:Some_Article", "Some_Article"},
		{"Talk:", ""},
		{"Foo", "Foo"}, // no prefix to strip
	}

	for _, tt := range tests {
		if got := SubjectPageURL(tt.url); got != tt.want {
			t.Errorf("SubjectPageURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestIsUserPage(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"User:Alice", true},
		{"User:alice_bob", true},
		{"User_talk:Alice", false},
		{"Talk:User:Alice", false},
		{"Alice", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsUserPage(tt.url); got != tt.want {
			t.Errorf("IsUserPage(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestIsUserTalkPage(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"User_talk:Alice", true},
		{"User:Alice", false},
		{"Talk:Alice", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsUserTalkPage(tt.url); got != tt.want {
			t.Errorf("IsUserTalkPage(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestUserPageScreenName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"User:Alice", "Alice"},
		{"User_talk:Alice", "Alice"},
		{"User:alice_bob", "alice_bob"},
		{"User_talk:alice_bob", "alice_bob"},
	}
	for _, tt := range tests {
		if got := UserPageScreenName(tt.url); got != tt.want {
			t.Errorf("UserPageScreenName(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestUserPageURL(t *testing.T) {
	if got := UserPageURL("Alice"); got != "User:Alice" {
		t.Errorf("UserPageURL(\"Alice\") = %q, want \"User:Alice\"", got)
	}
}

func TestUserTalkPageURL(t *testing.T) {
	if got := UserTalkPageURL("Alice"); got != "User_talk:Alice" {
		t.Errorf("UserTalkPageURL(\"Alice\") = %q, want \"User_talk:Alice\"", got)
	}
}
