package templater

import (
	"fmt"
	"net/url"

	"github.com/danielledeleo/periwiki/wiki"
)

// URL helper functions for templates.
// These generate article URLs with query parameters.

// articleURL returns the base URL for viewing an article.
// Example: articleURL("Test_Article") → "/wiki/Test_Article"
func articleURL(urlPath string) string {
	return "/wiki/" + url.PathEscape(urlPath)
}

// revisionURL returns a URL for viewing a specific revision of an article.
// Example: revisionURL("Test", 5) → "/wiki/Test?revision=5"
func revisionURL(urlPath string, revision int) string {
	return fmt.Sprintf("/wiki/%s?revision=%d", url.PathEscape(urlPath), revision)
}

// editURL returns a URL for editing an article.
// If revision is provided (non-zero), it's included for editing/restoring that revision.
// Example: editURL("Test", 0) → "/wiki/Test?edit"
// Example: editURL("Test", 5) → "/wiki/Test?edit&revision=5"
func editURL(urlPath string, revision int) string {
	if revision > 0 {
		return fmt.Sprintf("/wiki/%s?edit&revision=%d", url.PathEscape(urlPath), revision)
	}
	return fmt.Sprintf("/wiki/%s?edit", url.PathEscape(urlPath))
}

// historyURL returns a URL for viewing an article's revision history.
// Example: historyURL("Test") → "/wiki/Test?history"
func historyURL(urlPath string) string {
	return fmt.Sprintf("/wiki/%s?history", url.PathEscape(urlPath))
}

// diffURL returns a URL for viewing a diff between two revisions.
// If newRevision is 0, it means "diff to current" (omits the new param).
// Example: diffURL("Test", 3, 5) → "/wiki/Test?diff&old=3&new=5"
// Example: diffURL("Test", 3, 0) → "/wiki/Test?diff&old=3" (to current)
func diffURL(urlPath string, oldRevision, newRevision int) string {
	if newRevision > 0 {
		return fmt.Sprintf("/wiki/%s?diff&old=%d&new=%d", url.PathEscape(urlPath), oldRevision, newRevision)
	}
	return fmt.Sprintf("/wiki/%s?diff&old=%d", url.PathEscape(urlPath), oldRevision)
}

// isTalkPage returns true if the URL is in the Talk namespace.
func isTalkPage(urlPath string) bool {
	return wiki.IsTalkPage(urlPath)
}

// talkPageURL returns the Talk namespace URL for a given article URL.
func talkPageURL(urlPath string) string {
	return wiki.TalkPageURL(urlPath)
}

// subjectURL returns the subject article URL for a given Talk page URL.
func subjectURL(urlPath string) string {
	return wiki.SubjectPageURL(urlPath)
}
