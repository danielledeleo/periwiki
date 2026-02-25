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
