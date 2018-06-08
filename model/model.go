package model

import (
	"crypto/sha512"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jagger27/iwikii/pandoc"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/crypto/bcrypt"
)

type WikiModel struct {
	db db
	*Config
	sanitizer *bluemonday.Policy
	// cache, perhaps
}

type Config struct {
	MinimumPasswordLength int
}

type db interface {
	SelectArticle(url string) (*Article, error)
	SelectArticleByRevision(url string, hash string) (*Article, error)
	SelectRevision(hash string) (*Revision, error)
	SelectUserByScreenname(screenname string, withHash bool) (*User, error)
	SelectRevisionHistory(url string) ([]*Revision, error)
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

type Revision struct {
	ID           int    `db:"id"`
	Title        string `db:"title"`
	Markdown     string `db:"markdown"`
	HTML         string `db:"html"`
	Hash         string `db:"hashval"`
	Creator      *User
	Created      time.Time `db:"created"`
	PreviousHash string    `db:"previous_hash"`
	Comment      string    `db:"comment"`
}

type User struct {
	Email        string `db:"email"`
	ScreenName   string `db:"screenname"`
	ID           int    `db:"id"`
	PasswordHash string `db:"passwordhash"`
	RawPassword  string
	IPAddress    string
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

func New(db db, conf *Config, s *bluemonday.Policy) *WikiModel {
	return &WikiModel{db: db, Config: conf, sanitizer: s}
}

func (model *WikiModel) GetArticle(url string) (*Article, error) {
	article, err := model.db.SelectArticle(url)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return article, err
}

var ErrUsernameTaken = errors.New("username already in use")
var ErrEmailTaken = errors.New("email already in use")
var ErrPasswordTooShort = errors.New("password too short")
var ErrIncorrectPassword = errors.New("incorrect password")
var ErrUsernameNotFound = errors.New("username not found")
var ErrArticleNotModified = errors.New("article not modified")

func (model *WikiModel) PostArticle(article *Article) error {
	x := sha512.Sum384([]byte(article.Title + article.Markdown))
	article.Hash = base64.URLEncoding.EncodeToString(x[:])

	sourceRevision, err := model.GetArticleByRevisionHash(article.URL, article.PreviousHash)
	if err != sql.ErrNoRows {
		if sourceRevision.Hash == article.Hash {
			return ErrArticleNotModified
		} else if err != nil {
			return err
		}
	}

	unsafe, err := pandoc.MarkdownToHTML(article.Markdown)
	if err != nil {
		return err
	}

	article.HTML = model.sanitizer.Sanitize(string(unsafe))
	err = model.db.InsertArticle(article)

	return err
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

func AnonymousUser() *User {
	return &User{ID: 0}
}

func (model *WikiModel) PreviewMarkdown(markdown string) (string, error) {
	unsafe, err := pandoc.MarkdownToHTML(markdown)
	if err != nil {
		return "", err
	}

	return model.sanitizer.Sanitize(string(unsafe)), nil
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

func (model *WikiModel) GetArticleByRevisionHash(url string, hash string) (*Article, error) {
	revision, err := model.db.SelectArticleByRevision(url, hash)
	if err != nil {
		return nil, err
	}
	return revision, err
}

func (model *WikiModel) GetRevisionHistory(url string) ([]*Revision, error) {
	return model.db.SelectRevisionHistory(url)
}
