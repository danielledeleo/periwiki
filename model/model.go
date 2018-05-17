package model

import "time"

type WikiModel struct {
	db
}

type db interface {
	GetArticle(url string) (*Article, error)
	// SetArticle(*Article) error
	// UpdateArticle(*Article) error
}

type Article struct {
	URL       string
	Title     string
	Revisions *RevisionTree
}

type RevisionTree interface {
	Head() *Revision
	Add(*Revision)
	// Compare(a, b *Revision) (diff string) ?
}

type SimpleRevisionTree struct {
	head      *Revision
	revisions []*Revision
}

func (rt *SimpleRevisionTree) Head() *Revision {
	return rt.head
}

func (rt *SimpleRevisionTree) Add(revision *Revision) {
	if rt.head != nil {
		rt.head = revision
	}
	if rt.revisions == nil {
		rt.revisions = make([]*Revision, 0)
	}

	rt.revisions = append(rt.revisions, revision)
}

type Revision struct {
	Markdown string
	HTML     string
	Hash     string
	Creator  *User
	Date     time.Time
	Parent   *Revision
}

type User struct {
	Email        string
	ScreenName   string
	ID           int
	PasswordHash []byte
	// Role
}
