package special

import (
	"net/http"
	"testing"
)

// mockHandler is a simple handler for testing.
type mockHandler struct {
	called bool
}

func (m *mockHandler) Handle(rw http.ResponseWriter, req *http.Request) {
	m.called = true
}

func TestRegistry(t *testing.T) {
	t.Run("Get unregistered returns false", func(t *testing.T) {
		reg := NewRegistry()
		_, ok := reg.Get("NonExistent")
		if ok {
			t.Error("expected false for unregistered page")
		}
	})

	t.Run("Register and Get", func(t *testing.T) {
		reg := NewRegistry()
		mock := &mockHandler{}
		reg.Register("Test", mock)
		handler, ok := reg.Get("Test")
		if !ok {
			t.Error("expected true for registered page")
		}
		if handler != mock {
			t.Error("expected to retrieve same handler")
		}
	})

	t.Run("case sensitive", func(t *testing.T) {
		reg := NewRegistry()
		mock := &mockHandler{}
		reg.Register("Test", mock)

		_, ok := reg.Get("test")
		if ok {
			t.Error("expected case-sensitive lookup to fail for 'test'")
		}

		_, ok = reg.Get("TEST")
		if ok {
			t.Error("expected case-sensitive lookup to fail for 'TEST'")
		}

		_, ok = reg.Get("Test")
		if !ok {
			t.Error("expected exact case match to succeed")
		}
	})

	t.Run("overwrite existing", func(t *testing.T) {
		reg := NewRegistry()
		mock1 := &mockHandler{}
		mock2 := &mockHandler{}

		reg.Register("Test", mock1)
		reg.Register("Test", mock2)

		handler, ok := reg.Get("Test")
		if !ok {
			t.Error("expected handler to exist")
		}
		if handler != mock2 {
			t.Error("expected second handler to overwrite first")
		}
	})
}
