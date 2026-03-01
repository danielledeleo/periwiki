package wiki_test

import (
	"testing"

	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
)

func TestGetOrCreateSetting(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	db := app.RawDB

	t.Run("creates setting when missing", func(t *testing.T) {
		val, err := wiki.GetOrCreateSetting(db, "test_key", func() string {
			return "default_value"
		})
		if err != nil {
			t.Fatalf("GetOrCreateSetting failed: %v", err)
		}
		if val != "default_value" {
			t.Errorf("expected 'default_value', got %q", val)
		}

		// Verify it was persisted
		var stored string
		if err := db.QueryRow("SELECT value FROM Setting WHERE key = 'test_key'").Scan(&stored); err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if stored != "default_value" {
			t.Errorf("expected persisted 'default_value', got %q", stored)
		}
	})

	t.Run("returns existing setting without calling defaultFn", func(t *testing.T) {
		if _, err := db.Exec("INSERT INTO Setting (key, value) VALUES ('existing', 'original')"); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		called := false
		val, err := wiki.GetOrCreateSetting(db, "existing", func() string {
			called = true
			return "should_not_be_used"
		})
		if err != nil {
			t.Fatalf("GetOrCreateSetting failed: %v", err)
		}
		if val != "original" {
			t.Errorf("expected 'original', got %q", val)
		}
		if called {
			t.Error("defaultFn should not be called when setting exists")
		}
	})
}

func TestUpdateSetting(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	db := app.RawDB

	t.Run("updates existing setting", func(t *testing.T) {
		if _, err := db.Exec("INSERT INTO Setting (key, value) VALUES ('key', 'old')"); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		if err := wiki.UpdateSetting(db, "key", "new"); err != nil {
			t.Fatalf("UpdateSetting failed: %v", err)
		}

		var val string
		if err := db.QueryRow("SELECT value FROM Setting WHERE key = 'key'").Scan(&val); err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if val != "new" {
			t.Errorf("expected 'new', got %q", val)
		}
	})

	t.Run("creates setting when missing", func(t *testing.T) {
		if err := wiki.UpdateSetting(db, "new_key", "value"); err != nil {
			t.Fatalf("UpdateSetting failed: %v", err)
		}

		var val string
		if err := db.QueryRow("SELECT value FROM Setting WHERE key = 'new_key'").Scan(&val); err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if val != "value" {
			t.Errorf("expected 'value', got %q", val)
		}
	})
}
