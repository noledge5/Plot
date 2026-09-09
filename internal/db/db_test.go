package db

import (
	"testing"
)

func TestOpenMigrateAndSettings(t *testing.T) {
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	// Zweiter Aufruf darf nicht erneut migrieren.
	var version int
	if err := d.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("schema_version: %v", err)
	}
	if version != len(migrations) {
		t.Fatalf("Schemastand %d, erwartet %d", version, len(migrations))
	}
	if err := d.migrate(); err != nil {
		t.Fatalf("erneute Migration: %v", err)
	}

	if got := d.Setting("openrouter_key", "leer"); got != "leer" {
		t.Fatalf("Vorgabewert nicht zurückgegeben: %q", got)
	}
	if err := d.SetSetting("openrouter_key", "sk-test"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := d.SetSetting("openrouter_key", "sk-zwei"); err != nil {
		t.Fatalf("SetSetting überschreiben: %v", err)
	}
	if got := d.Setting("openrouter_key", ""); got != "sk-zwei" {
		t.Fatalf("Setting = %q, erwartet sk-zwei", got)
	}
}

// FTS5 wird erst ab M3 gebraucht. Der Test steht jetzt schon hier, damit sich
// früh zeigt, ob die reine Go-SQLite-Bibliothek es mitbringt.
func TestFTS5Verfuegbar(t *testing.T) {
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	if _, err := d.Exec(`CREATE VIRTUAL TABLE probe USING fts5(text)`); err != nil {
		t.Fatalf("FTS5 nicht verfügbar: %v", err)
	}
	if _, err := d.Exec(`INSERT INTO probe (text) VALUES ('Mira hat den Brief nicht erwähnt')`); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	var n int
	if err := d.QueryRow(`SELECT count(*) FROM probe WHERE probe MATCH 'Brief'`).Scan(&n); err != nil {
		t.Fatalf("MATCH: %v", err)
	}
	if n != 1 {
		t.Fatalf("Treffer = %d, erwartet 1", n)
	}
}
