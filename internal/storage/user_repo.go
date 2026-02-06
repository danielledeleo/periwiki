package storage

import (
	"fmt"
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

	result, err := tx.Exec(`INSERT INTO User(screenname, email) VALUES (?, ?)`, user.ScreenName, user.Email)

	if err != nil {
		if err.Error() == "UNIQUE constraint failed: User.screenname" {
			return wiki.ErrUsernameTaken
		} else if err.Error() == "UNIQUE constraint failed: User.email" {
			return wiki.ErrEmailTaken
		}
		return
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return
	}
	user.ID = int(userID)

	if _, err = tx.Exec(`INSERT INTO Password(user_id, passwordhash) VALUES (?, ?)`, user.ID, user.PasswordHash); err != nil {
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

func (db *sqliteDb) SelectUserByID(id int) (*wiki.User, error) {
	user := &wiki.User{}
	err := db.conn.Get(user, `SELECT id, screenname, email, role, created_at FROM User WHERE id = ?`, id)
	return user, err
}

func (db *sqliteDb) SelectAllUsers() ([]*wiki.User, error) {
	var users []*wiki.User
	err := db.conn.Select(&users, `SELECT id, screenname, email, role, created_at FROM User WHERE id != 0 ORDER BY id`)
	return users, err
}

func (db *sqliteDb) UpdateUserRole(id int, role string) error {
	if role != wiki.RoleAdmin && role != wiki.RoleUser {
		return fmt.Errorf("invalid role: %s", role)
	}
	_, err := db.conn.Exec(`UPDATE User SET role = ? WHERE id = ?`, role, id)
	return err
}
