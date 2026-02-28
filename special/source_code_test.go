package special

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielledeleo/periwiki/internal/embedded"
)

func TestSourceCodePage(t *testing.T) {
	handler := NewSourceCodePage()

	req := httptest.NewRequest("GET", "/wiki/Special:SourceCode", nil)
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/gzip" {
		t.Errorf("expected Content-Type application/gzip, got %q", ct)
	}

	if cd := rr.Header().Get("Content-Disposition"); cd != "attachment; filename=periwiki-source.tar.gz" {
		t.Errorf("expected Content-Disposition attachment, got %q", cd)
	}

	if rr.Body.Len() != len(embedded.SourceTarball) {
		t.Errorf("expected body length %d, got %d", len(embedded.SourceTarball), rr.Body.Len())
	}
}
