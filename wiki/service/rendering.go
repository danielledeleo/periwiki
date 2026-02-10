package service

import (
	"github.com/danielledeleo/periwiki/render"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/microcosm-cc/bluemonday"
)

// RenderingService defines the interface for rendering markdown content.
type RenderingService interface {
	// Render converts markdown to sanitized HTML.
	Render(markdown string) (string, error)

	// PreviewMarkdown renders markdown for preview purposes.
	PreviewMarkdown(markdown string) (string, error)
}

// renderingService is the default implementation of RenderingService.
type renderingService struct {
	renderer  *render.HTMLRenderer
	sanitizer *bluemonday.Policy
}

// NewRenderingService creates a new RenderingService.
func NewRenderingService(renderer *render.HTMLRenderer, sanitizer *bluemonday.Policy) RenderingService {
	return &renderingService{
		renderer:  renderer,
		sanitizer: sanitizer,
	}
}

// Render converts markdown to sanitized HTML.
func (s *renderingService) Render(markdown string) (string, error) {
	// Strip frontmatter before rendering
	fm, content := wiki.ParseFrontmatter(markdown)

	skipTOC := fm.TOC != nil && !*fm.TOC
	unsafe, err := s.renderer.Render(content, skipTOC)
	if err != nil {
		return "", err
	}

	return s.sanitizer.Sanitize(string(unsafe)), nil
}

// PreviewMarkdown renders markdown for preview purposes.
func (s *renderingService) PreviewMarkdown(markdown string) (string, error) {
	return s.Render(markdown)
}
