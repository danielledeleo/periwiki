package wiki

import "time"

// Revision represents a specific version of an article.
type Revision struct {
	ID         int    `db:"id"`
	Title      string `db:"title"`
	Markdown   string `db:"markdown"`
	HTML       string `db:"html"`
	Hash       string `db:"hashval"`
	Creator    *User
	Created    time.Time `db:"created"`
	PreviousID int       `db:"previous_id"`
	Comment    string    `db:"comment"`
}
