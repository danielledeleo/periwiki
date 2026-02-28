package special

import (
	"net/http"

	"github.com/danielledeleo/periwiki/internal/embedded"
)

// SourceCodePage serves the embedded source tarball as a download.
type SourceCodePage struct{}

// NewSourceCodePage creates a new SourceCode special page handler.
func NewSourceCodePage() *SourceCodePage {
	return &SourceCodePage{}
}

// Handle serves the source tarball as a gzip download.
func (p *SourceCodePage) Handle(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/gzip")
	rw.Header().Set("Content-Disposition", "attachment; filename=periwiki-source.tar.gz")
	rw.Write(embedded.SourceTarball)
}
