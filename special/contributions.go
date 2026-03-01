package special

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielledeleo/periwiki/wiki"
)

// ContributionsLister retrieves contributions for a user.
type ContributionsLister interface {
	GetRevisionsByScreenName(screenName string) ([]*wiki.ContributionEntry, error)
}

// ContributionsTemplater renders the contributions template.
type ContributionsTemplater interface {
	RenderTemplate(w io.Writer, name string, base string, data map[string]interface{}) error
}

// UserChecker verifies a user exists.
type UserChecker interface {
	GetUserByScreenName(screenName string) (*wiki.User, error)
}

// ContributionsPage handles Special:Contributions requests.
type ContributionsPage struct {
	contributions ContributionsLister
	users         UserChecker
	templater     ContributionsTemplater
}

// NewContributionsPage creates a new contributions special page handler.
func NewContributionsPage(contributions ContributionsLister, users UserChecker, templater ContributionsTemplater) *ContributionsPage {
	return &ContributionsPage{
		contributions: contributions,
		users:         users,
		templater:     templater,
	}
}

// Handle serves the contributions page for a user.
// URL format: /wiki/Special:Contributions/ScreenName
func (p *ContributionsPage) Handle(rw http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	const prefix = "/wiki/Special:Contributions/"
	screenName := ""
	if idx := strings.Index(path, prefix); idx >= 0 {
		screenName = path[idx+len(prefix):]
	}

	if screenName == "" {
		http.Error(rw, "Usage: Special:Contributions/Username", http.StatusBadRequest)
		return
	}

	// Verify user exists
	if _, err := p.users.GetUserByScreenName(screenName); err != nil {
		http.Error(rw, "User not found", http.StatusNotFound)
		return
	}

	entries, err := p.contributions.GetRevisionsByScreenName(screenName)
	if err != nil {
		slog.Error("failed to get contributions", "screenName", screenName, "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Page":          wiki.NewStaticPage("Contributions by " + screenName),
		"Context":       req.Context(),
		"ScreenName":    screenName,
		"Contributions": entries,
	}

	if err := p.templater.RenderTemplate(rw, "contributions.html", "index.html", data); err != nil {
		slog.Error("failed to render contributions template", "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
	}
}
