package storage

import (
	"log/slog"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/jmoiron/sqlx"
)

// User repository methods for sqliteDb

func (db *sqliteDb) InsertUser(user *wiki.User) (err error) {
	var tx *sqlx.Tx
	tx, err = db.conn.Beginx()

	defer func() {
		if err != nil {
			slog.Error("user insert failed", "error", err)
			if rbErr := tx.Rollback(); rbErr != nil {
				slog.Error("transaction rollback failed", "error", rbErr)
			}
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				slog.Error("transaction commit failed", "error", commitErr)
			}
		}
	}()

	_, err = tx.Exec(`INSERT INTO User(screenname, email) VALUES (?, ?)`, user.ScreenName, user.Email)

	if err != nil {
		if err.Error() == "UNIQUE constraint failed: User.screenname" {
			return wiki.ErrUsernameTaken
		} else if err.Error() == "UNIQUE constraint failed: User.email" {
			return wiki.ErrEmailTaken
		}
		return
	}
	if _, err = tx.Exec(`INSERT INTO Password(user_id, passwordhash) VALUES (last_insert_rowid(), ?)`, user.PasswordHash); err != nil {
		return
	}

	return nil
}

func (db *sqliteDb) SelectUserByScreenname(screenname string, withHash bool) (*wiki.User, error) {
	user := &wiki.User{}

	var err error
	if withHash {
		err = db.SelectUserScreennameWithHashStmt.Get(user, screenname)
	} else {
		err = db.SelectUserScreennameStmt.Get(user, screenname)
	}

	return user, err
}
