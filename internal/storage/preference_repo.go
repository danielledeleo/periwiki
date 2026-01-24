package storage

import (
	"database/sql"

	"github.com/danielledeleo/periwiki/wiki"
)

// Preference repository methods for sqliteDb

func (db *sqliteDb) InsertPreference(pref *wiki.Preference) error {
	_, err := db.conn.Exec(`INSERT OR REPLACE INTO Preference (pref_label, pref_type, help_text, pref_int, pref_text, pref_selection)
		VALUES(?, ?, ?, ?, ?, ?)`,
		pref.Label,
		pref.Type,
		pref.HelpText,
		pref.IntValue,
		pref.TextValue,
		pref.SelectionValue,
	)
	return err
}

func (db *sqliteDb) SelectPreference(key string) (*wiki.Preference, error) {
	pref := &wiki.Preference{}
	err := db.conn.Select(`SELECT * FROM Preference WHERE pref_label = ?`, key)
	if err == sql.ErrNoRows {
		return nil, wiki.ErrGenericNotFound
	} else if err != nil {
		return nil, err
	}
	return pref, err
}
