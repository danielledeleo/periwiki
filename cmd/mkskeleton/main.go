// mkskeleton creates a skeleton SQLite database from schema.sql for use by
// sqlboiler's code generation.
package main

import (
	"database/sql"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	const (
		schemaPath = "internal/storage/schema.sql"
		dbPath     = "internal/storage/skeleton.db"
	)

	os.Remove(dbPath)

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(string(schema)); err != nil {
		log.Fatal(err)
	}
}
