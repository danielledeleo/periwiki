package model

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gorilla/sessions"
)

type WikiModel struct {
	db db
	*Config
	// cache, perhaps
}

type Config struct {
	MinimumPasswordLength int
}

type db interface {
	SelectArticle(url string) (*Article, error)
	SelectUserByScreenname(screenname string, withHash bool) (*User, error)
	InsertArticle(article *Article) error
	InsertUser(user *User) error

	// For cookie store, delete isn't part of the interface for some reason
	sessions.Store
	Delete(r *http.Request, rw http.ResponseWriter, s *sessions.Session) error

	// SetArticle(*Article) error
	// UpdateArticle(*Article) error
}

type Article struct {
	URL string
	*Revision
}

func (article *Article) String() string {
	return fmt.Sprintf("%q %q", article.URL, *article.Revision)
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
	Title    string `db:"title"`
	Markdown string `db:"markdown"`
	HTML     string `db:"html"`
	Hash     string `db:"hashval"`
	Creator  *User
	Created  time.Time `db:"created"`
	Previous *Revision
}

type User struct {
	Email        string `db:"email"`
	ScreenName   string `db:"screenname"`
	ID           int    `db:"id"`
	PasswordHash string `db:"passwordhash"`
	RawPassword  string
	// Role
}

func (u *User) SetPasswordHash() error {
	rawHash, err := bcrypt.GenerateFromPassword([]byte(u.RawPassword), bcrypt.MinCost)
	u.RawPassword = ""
	if err != nil {
		return err
	}
	u.PasswordHash = string(rawHash)
	return nil
}

func New(db db, conf *Config) *WikiModel {
	return &WikiModel{db: db, Config: conf}
}

func (model *WikiModel) GetArticle(url string) (*Article, error) {
	article, err := model.db.SelectArticle(url)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	// article.HTML = article.Markdown // TODO: Parse markdown into clean HTML
	return article, err
}

var ErrUsernameTaken = errors.New("username already in use")
var ErrEmailTaken = errors.New("email already in use")
var ErrPasswordTooShort = errors.New("password too short")
var ErrIncorrectPassword = errors.New("incorrect password")
var ErrUsernameNotFound = errors.New("username not found")

func (model *WikiModel) PostArticle(article *Article) error {
	return model.db.InsertArticle(article)
}

// PostUser inserts attempts to insert a user into the database
func (model *WikiModel) PostUser(user *User) error {
	if len(user.RawPassword) < model.MinimumPasswordLength {
		return errors.New(ErrPasswordTooShort.Error() + fmt.Sprintf(" (must be %d characters long)", model.MinimumPasswordLength))
	}
	err := user.SetPasswordHash()
	if err != nil {
		return err
	}
	return model.db.InsertUser(user)
}

// GetCookie wraps gorilla/sessions.Store.Get, as implemented by WikiModel.db
func (model *WikiModel) GetCookie(r *http.Request, name string) (*sessions.Session, error) {
	return model.db.Get(r, name)
}

// NewCookie wraps gorilla/sessions.Store.New, as implemented by WikiModel.db
func (model *WikiModel) NewCookie(r *http.Request, name string) (*sessions.Session, error) {
	return model.db.New(r, name)
}

// SaveCookie wraps gorilla/sessions.Store.Save, as implemented by WikiModel.db
func (model *WikiModel) SaveCookie(r *http.Request, rw http.ResponseWriter, s *sessions.Session) error {
	return model.db.Save(r, rw, s)
}

// DeleteCookie wraps gorilla/sessions.Store.Delete, as implemented by WikiModel.db
func (model *WikiModel) DeleteCookie(r *http.Request, rw http.ResponseWriter, s *sessions.Session) error {
	return model.db.Delete(r, rw, s)
}

func (model *WikiModel) CheckUserPassword(u *User) error {
	dbUser, err := model.db.SelectUserByScreenname(u.ScreenName, true)
	if err == sql.ErrNoRows {
		return ErrUsernameNotFound
	}
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(u.RawPassword))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return ErrIncorrectPassword
	}
	return err
}

func (model *WikiModel) GetUserByScreenName(screenname string) (*User, error) {
	dbUser, err := model.db.SelectUserByScreenname(screenname, false)
	if err == sql.ErrNoRows {
		return nil, ErrUsernameNotFound
	}
	return dbUser, err
}
