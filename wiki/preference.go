package wiki

import "database/sql"

// Preference type constants
const (
	IntPref = iota
	TextPref
	SelectionPref
)

// Preference represents a wiki configuration preference.
type Preference struct {
	ID               int            `db:"id"`
	Label            string         `db:"pref_label"`
	Type             int            `db:"pref_type"`
	HelpText         sql.NullString `db:"help_text"`
	IntValue         sql.NullInt64  `db:"pref_int"`
	TextValue        sql.NullString `db:"pref_text"`
	SelectionValue   sql.NullInt64  `db:"pref_selection"`
	SelectionChoices []*PreferenceSelection
}

// PreferenceSelection represents a selectable option for a preference.
type PreferenceSelection struct {
	PreferenceID int    `db:"pref_id"`
	Value        int    `db:"val"`
	Label        string `db:"pref_selection_label"`
}
