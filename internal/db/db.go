// Package db kapselt die SQLite-Datenbank. Alles liegt in einer Datei im
// Datenordner; ein Backup dieser Datei sichert den kompletten Zustand.
package db

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

// Open öffnet die Datenbank im Datenordner und bringt das Schema auf Stand.
func Open(dataDir string) (*DB, error) {
	path := filepath.Join(dataDir, "plot.sqlite")
	// WAL überlebt Abstürze sauber, busy_timeout verhindert Fehler, wenn der
	// Hintergrund-Indexer gleichzeitig schreibt.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("datenbank öffnen: %w", err)
	}
	// Eine NAS mit 1 GB RAM braucht keine Verbindungsflut, und SQLite schreibt
	// ohnehin seriell.
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(2)

	d := &DB{sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return d, nil
}

// migrations läuft strikt der Reihe nach; angewandte Schritte stehen in
// schema_version. Neue Schritte werden nur angehängt, nie geändert.
var migrations = []string{
	`
CREATE TABLE users (
	id         INTEGER PRIMARY KEY,
	name       TEXT NOT NULL UNIQUE,
	pwhash     TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE sessions (
	token      TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL
);

-- Anwendungsweite Einstellungen, u.a. der OpenRouter-Key. Bewusst in der
-- Datenbank statt in einer Datei: der Nutzer soll nie einen Texteditor auf
-- der NAS brauchen.
CREATE TABLE settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE story (
	id            INTEGER PRIMARY KEY,
	title         TEXT NOT NULL,
	system_prompt TEXT NOT NULL DEFAULT '',
	settings_json TEXT NOT NULL DEFAULT '{}',
	head_node_id  INTEGER,
	created_at    TEXT NOT NULL,
	updated_at    TEXT NOT NULL
);

-- Der Turn-Baum. parent_id NULL heißt Wurzel. Regenerieren erzeugt einen
-- Geschwisterknoten, Bearbeiten einen neuen Zweig - nichts wird gelöscht.
CREATE TABLE node (
	id         INTEGER PRIMARY KEY,
	story_id   INTEGER NOT NULL REFERENCES story(id) ON DELETE CASCADE,
	parent_id  INTEGER REFERENCES node(id) ON DELETE CASCADE,
	kind       TEXT NOT NULL,              -- turn | narration | note
	role       TEXT NOT NULL,              -- user | assistant
	content    TEXT NOT NULL,
	model      TEXT NOT NULL DEFAULT '',
	usage_json TEXT NOT NULL DEFAULT '{}',
	flags_json TEXT NOT NULL DEFAULT '{}', -- Befunde der Autonomie-Schicht (ab M2)
	created_at TEXT NOT NULL
);
CREATE INDEX idx_node_story  ON node(story_id);
CREATE INDEX idx_node_parent ON node(parent_id);

CREATE TABLE save_slot (
	id         INTEGER PRIMARY KEY,
	story_id   INTEGER NOT NULL REFERENCES story(id) ON DELETE CASCADE,
	node_id    INTEGER NOT NULL REFERENCES node(id) ON DELETE CASCADE,
	name       TEXT NOT NULL,
	kind       TEXT NOT NULL DEFAULT 'manual', -- manual | auto
	preview    TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX idx_slot_story ON save_slot(story_id);

-- Jede Anfrage an OpenRouter mit exaktem Payload. Ohne das lässt sich nie
-- klären, ob eine schlechte Antwort am Modell oder am Prompt lag.
CREATE TABLE run_log (
	id            INTEGER PRIMARY KEY,
	story_id      INTEGER REFERENCES story(id) ON DELETE CASCADE,
	node_id       INTEGER REFERENCES node(id) ON DELETE SET NULL,
	purpose       TEXT NOT NULL,          -- narrate | compare | utility
	model         TEXT NOT NULL,
	request_json  TEXT NOT NULL,
	response_json TEXT NOT NULL DEFAULT '{}',
	cost_usd      REAL NOT NULL DEFAULT 0,
	latency_ms    INTEGER NOT NULL DEFAULT 0,
	error         TEXT NOT NULL DEFAULT '',
	created_at    TEXT NOT NULL
);
CREATE INDEX idx_runlog_story ON run_log(story_id);
CREATE INDEX idx_runlog_node  ON run_log(node_id);
`,
	// M2: Figuren mit Grenzen und Antrieben als eigene Felder. Die
	// Autonomie-Schicht braucht sie strukturiert, nicht als Prosa im Blatt -
	// Fließtext wird vom Modell überlesen, eine Liste nicht.
	`
CREATE TABLE character (
	id         INTEGER PRIMARY KEY,
	story_id   INTEGER NOT NULL REFERENCES story(id) ON DELETE CASCADE,
	rolle      TEXT NOT NULL DEFAULT 'npc',  -- npc | persona (die Figur des Spielers)
	name       TEXT NOT NULL,
	sheet_json TEXT NOT NULL DEFAULT '{}',
	aktiv      INTEGER NOT NULL DEFAULT 1,   -- steht diese Figur gerade in der Szene?
	sortierung INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX idx_character_story ON character(story_id);
`,
	// M3: Gedächtnis. Die Chronik ersetzt, was der Verlaufsdeckel abschneidet;
	// das Faktenblatt hält fest, was dauerhaft gilt. Beide hängen am Knoten,
	// damit sie in einem verworfenen Zweig nicht mitgelten.
	`
CREATE TABLE memory_summary (
	id         INTEGER PRIMARY KEY,
	story_id   INTEGER NOT NULL REFERENCES story(id) ON DELETE CASCADE,
	ebene      INTEGER NOT NULL DEFAULT 1,   -- 1 = Abschnitt, 2 = Kapitel
	von_node   INTEGER NOT NULL REFERENCES node(id) ON DELETE CASCADE,
	bis_node   INTEGER NOT NULL REFERENCES node(id) ON DELETE CASCADE,
	text       TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX idx_summary_story ON memory_summary(story_id);

CREATE TABLE memory_fact (
	id         INTEGER PRIMARY KEY,
	story_id   INTEGER NOT NULL REFERENCES story(id) ON DELETE CASCADE,
	betrifft   TEXT NOT NULL DEFAULT '',     -- Name der Figur, um die es geht
	text       TEXT NOT NULL,
	gewicht    INTEGER NOT NULL DEFAULT 50,  -- wie wichtig, 0-100
	status     TEXT NOT NULL DEFAULT 'canon',-- proposed | canon | retired
	angeheftet INTEGER NOT NULL DEFAULT 0,   -- immer in den Prompt
	quelle     INTEGER REFERENCES node(id) ON DELETE SET NULL,
	gilt_ab    INTEGER REFERENCES node(id) ON DELETE SET NULL,
	gilt_bis   INTEGER REFERENCES node(id) ON DELETE SET NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX idx_fact_story ON memory_fact(story_id, status);

-- Volltextindex für den Abruf. Reine Stichwortsuche trägt im Rollenspiel
-- weit, weil am häufigsten nach Eigennamen, Zitaten und Daten gesucht wird.
-- Die rowid entspricht memory_fact.id, damit Einfügen und Löschen einfach
-- bleiben; eine inhaltslose Tabelle wäre sparsamer, aber umständlich zu
-- pflegen, und ein paar hundert Fakten wiegen nichts.
CREATE VIRTUAL TABLE memory_fts USING fts5(text, betrifft);
`,
	// M4: Beziehungen. Die Werte werden nicht gespeichert, sondern aus den
	// Deltas entlang des Pfades gefaltet - so gilt in einem verworfenen Zweig
	// auch der Zustand nicht, der dort entstanden ist.
	`
CREATE TABLE rel_delta (
	id         INTEGER PRIMARY KEY,
	story_id   INTEGER NOT NULL REFERENCES story(id) ON DELETE CASCADE,
	node_id    INTEGER NOT NULL REFERENCES node(id) ON DELETE CASCADE,
	figur      TEXT NOT NULL,              -- wer empfindet
	achse      TEXT NOT NULL,
	delta      INTEGER NOT NULL,
	begruendung TEXT NOT NULL DEFAULT '',
	zitat      TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX idx_reldelta_story ON rel_delta(story_id);
CREATE INDEX idx_reldelta_node  ON rel_delta(node_id);
`,
	// Die Figurenbibliothek. Eine Figur gehört sonst genau einer Geschichte;
	// wer dieselbe Person in einer zweiten Geschichte haben will, tippt ihr
	// Blatt bisher neu ab - und bekommt eine Figur, die sich anders verhält.
	// Hier liegt die Grundbeschreibung einmal, geschichtsunabhängig.
	//
	// Nur das Blatt wird geteilt, nicht der Beziehungsstand: Der entsteht aus
	// den Deltas entlang eines Pfades und gehört zu genau einem Spielverlauf.
	`
CREATE TABLE figur_vorlage (
	id         INTEGER PRIMARY KEY,
	name       TEXT NOT NULL,
	rolle      TEXT NOT NULL DEFAULT 'npc',  -- npc | persona
	sheet_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
-- Ein Name je Rolle. Zweimal dieselbe Figur in der Bibliothek heißt zwei
-- Fassungen, die auseinanderlaufen - genau das soll sie verhindern.
CREATE UNIQUE INDEX idx_vorlage_name ON figur_vorlage(name, rolle);
`,
}

func (d *DB) migrate() error {
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("versionstabelle: %w", err)
	}
	var current int
	if err := d.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current); err != nil {
		return fmt.Errorf("schemastand lesen: %w", err)
	}
	for i := current; i < len(migrations); i++ {
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_version (version) VALUES (?)`, i+1); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Setting liest einen Einstellungswert; fehlt er, kommt der Vorgabewert zurück.
func (d *DB) Setting(key, fallback string) string {
	var v string
	if err := d.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v); err != nil {
		return fallback
	}
	return v
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
	                  ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
