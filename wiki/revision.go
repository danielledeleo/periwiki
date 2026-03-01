package wiki

import "time"

// Revision represents a specific version of an article.
type Revision struct {
	ID         int       `db:"id"`
	Markdown   string    `db:"markdown"`
	HTML       string    `db:"html"`
	Hash       string    `db:"hashval"`
	Creator    *User
	Created    time.Time `db:"created"`
	PreviousID int       `db:"previous_id"`
	Comment    string    `db:"comment"`
}

// ContributionEntry represents a single edit by a user, for the contributions page.
type ContributionEntry struct {
	ArticleURL   string
	RevisionID   int
	PreviousID   int
	Created      time.Time
	Comment      string
	MarkdownSize int
}
