//go:build js

package storage

import (
	"database/sql"

	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func init() {
	sql.Register("sqlite", &driver.SQLite{})
}
