package db

import (
	"database/sql"
	"errors"
	"strings"
)

// --- Chronik ---

// Summary fasst einen Abschnitt zusammen. Sie ersetzt das, was der
// Verlaufsdeckel abschneidet, statt es verschwinden zu lassen.
type Summary struct {
	ID        int64  `json:"id"`
	StoryID   int64  `json:"storyId"`
	Ebene     int    `json:"ebene"`
	VonNode   int64  `json:"vonNode"`
	BisNode   int64  `json:"bisNode"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

func (d *DB) CreateSummary(storyID int64, ebene int, vonNode, bisNode int64, text string) (*Summary, error) {
	res, err := d.Exec(`INSERT INTO memory_summary (story_id, ebene, von_node, bis_node, text, created_at)
	                    VALUES (?, ?, ?, ?, ?, ?)`, storyID, ebene, vonNode, bisNode, text, now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Summary{ID: id, StoryID: storyID, Ebene: ebene, VonNode: vonNode,
		BisNode: bisNode, Text: text, CreatedAt: now()}, nil
}

// Summaries liefert die Zusammenfassungen einer Geschichte, älteste zuerst.
// Gefiltert wird erst beim Zusammenbau des Prompts: nur was auf dem
// aktuellen Pfad liegt, gilt auch.
func (d *DB) Summaries(storyID int64) ([]Summary, error) {
	rows, err := d.Query(`SELECT id, story_id, ebene, von_node, bis_node, text, created_at
	                      FROM memory_summary WHERE story_id = ? ORDER BY id`, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Summary{}
	for rows.Next() {
		var s Summary
		if err := rows.Scan(&s.ID, &s.StoryID, &s.Ebene, &s.VonNode, &s.BisNode, &s.Text, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LetzteZusammengefasst sagt, bis zu welchem Knoten die Chronik reicht.
func (d *DB) LetzteZusammengefasst(storyID int64) (int64, error) {
	var bis sql.NullInt64
	err := d.QueryRow(`SELECT MAX(bis_node) FROM memory_summary WHERE story_id = ?`, storyID).Scan(&bis)
	if err != nil {
		return 0, err
	}
	return bis.Int64, nil
}

func (d *DB) DeleteSummary(id int64) error {
	_, err := d.Exec(`DELETE FROM memory_summary WHERE id = ?`, id)
	return err
}

// --- Fakten ---

type Fact struct {
	ID         int64  `json:"id"`
	StoryID    int64  `json:"storyId"`
	Betrifft   string `json:"betrifft"`
	Text       string `json:"text"`
	Gewicht    int    `json:"gewicht"`
	Status     string `json:"status"` // proposed | canon | retired
	Angeheftet bool   `json:"angeheftet"`
	Quelle     *int64 `json:"quelle"`
	GiltAb     *int64 `json:"giltAb"`
	GiltBis    *int64 `json:"giltBis"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

const factSpalten = `id, story_id, betrifft, text, gewicht, status, angeheftet, quelle, gilt_ab, gilt_bis, created_at, updated_at`

func scanFact(sc interface{ Scan(...any) error }) (*Fact, error) {
	var f Fact
	var angeheftet int
	if err := sc.Scan(&f.ID, &f.StoryID, &f.Betrifft, &f.Text, &f.Gewicht, &f.Status,
		&angeheftet, &f.Quelle, &f.GiltAb, &f.GiltBis, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return nil, err
	}
	f.Angeheftet = angeheftet != 0
	return &f, nil
}

func (d *DB) CreateFact(f Fact) (*Fact, error) {
	if f.Gewicht <= 0 {
		f.Gewicht = 50
	}
	if f.Status == "" {
		f.Status = "canon"
	}
	ts := now()
	res, err := d.Exec(`INSERT INTO memory_fact
		(story_id, betrifft, text, gewicht, status, angeheftet, quelle, gilt_ab, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.StoryID, f.Betrifft, f.Text, f.Gewicht, f.Status, boolZahl(f.Angeheftet),
		f.Quelle, f.GiltAb, ts, ts)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if err := d.indexFact(id, f.Text, f.Betrifft); err != nil {
		return nil, err
	}
	return d.Fact(id)
}

func boolZahl(b bool) int {
	if b {
		return 1
	}
	return 0
}

// indexFact hält den Volltextindex mit dem Fakt gleichauf. Die rowid ist die
// Fakten-Kennung, deshalb genügt ein Löschen und Neuschreiben.
func (d *DB) indexFact(id int64, text, betrifft string) error {
	if _, err := d.Exec(`DELETE FROM memory_fts WHERE rowid = ?`, id); err != nil {
		return err
	}
	_, err := d.Exec(`INSERT INTO memory_fts (rowid, text, betrifft) VALUES (?, ?, ?)`, id, text, betrifft)
	return err
}

func (d *DB) Fact(id int64) (*Fact, error) {
	f, err := scanFact(d.QueryRow(`SELECT `+factSpalten+` FROM memory_fact WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

// Facts liefert alle Fakten einer Geschichte. Status "" heißt: alle.
func (d *DB) Facts(storyID int64, status string) ([]Fact, error) {
	q := `SELECT ` + factSpalten + ` FROM memory_fact WHERE story_id = ?`
	args := []any{storyID}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY angeheftet DESC, gewicht DESC, id DESC`

	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Fact{}
	for rows.Next() {
		f, err := scanFact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// stoppwoerter fliegen aus der Suche. Ohne sie ist eine ODER-Verknüpfung
// wertlos: "Was weißt du über den Bruder?" fände jeden Fakt, in dem "den"
// vorkommt.
var stoppwoerter = map[string]bool{
	"der": true, "die": true, "das": true, "den": true, "dem": true, "des": true,
	"ein": true, "eine": true, "einen": true, "einem": true, "einer": true, "eines": true,
	"und": true, "oder": true, "aber": true, "doch": true, "wenn": true, "dann": true,
	"als": true, "wie": true, "was": true, "wer": true, "wem": true, "wen": true,
	"wo": true, "wann": true, "warum": true, "dass": true, "damit": true,
	"ich": true, "due": true, "ihr": true, "wir": true, "sie": true, "er": true, "es": true,
	"mich": true, "dich": true, "sich": true, "uns": true, "euch": true,
	"ihm": true, "ihn": true, "ihnen": true, "mir": true, "dir": true,
	"ist": true, "sind": true, "war": true, "waren": true, "hat": true, "hatte": true,
	"haben": true, "habe": true, "wird": true, "werden": true, "wurde": true,
	"kann": true, "können": true, "soll": true, "sollte": true, "muss": true,
	"nicht": true, "kein": true, "keine": true, "auch": true, "noch": true, "nur": true,
	"schon": true, "sehr": true, "mehr": true, "alle": true, "man": true, "etwas": true,
	"von": true, "zum": true, "zur": true, "mit": true, "für": true, "auf": true,
	"aus": true, "bei": true, "nach": true, "über": true, "unter": true, "vor": true,
	"durch": true, "gegen": true, "ohne": true, "seit": true, "beim": true, "vom": true,
	"sein": true, "seine": true, "seinen": true, "seinem": true, "seiner": true,
	"ihre": true, "ihren": true, "ihrem": true, "ihrer": true,
	"mein": true, "meine": true, "dein": true, "deine": true,
	"hier": true, "dort": true, "jetzt": true, "immer": true, "wieder": true,
}

// SucheFakten findet Fakten per Stichwort. Der Suchtext wird entschärft:
// FTS5 hat eine eigene Abfragesprache, und ein Apostroph aus dem Erzähltext
// würde sie sonst zum Fehler bringen. Übrig bleiben die Wörter, die etwas
// aussagen - Eigennamen, Gegenstände, Begriffe.
func (d *DB) SucheFakten(storyID int64, suche string, grenze int) ([]Fact, error) {
	woerter := []string{}
	gesehen := map[string]bool{}
	for _, w := range strings.FieldsFunc(suche, func(r rune) bool {
		return !(r == '\'' || r > 127 || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}) {
		w = strings.Trim(w, "'")
		klein := strings.ToLower(w)
		if len([]rune(w)) < 3 || stoppwoerter[klein] || gesehen[klein] {
			continue
		}
		gesehen[klein] = true
		woerter = append(woerter, `"`+strings.ReplaceAll(w, `"`, "")+`"`)
	}
	if len(woerter) == 0 {
		return []Fact{}, nil
	}
	if grenze <= 0 {
		grenze = 20
	}

	rows, err := d.Query(`SELECT `+factSpalten+` FROM memory_fact
		WHERE id IN (SELECT rowid FROM memory_fts WHERE memory_fts MATCH ? ORDER BY rank LIMIT ?)
		  AND story_id = ? AND status != 'retired'`,
		strings.Join(woerter, " OR "), grenze, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Fact{}
	for rows.Next() {
		f, err := scanFact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

func (d *DB) UpdateFact(f Fact) error {
	_, err := d.Exec(`UPDATE memory_fact SET betrifft = ?, text = ?, gewicht = ?, status = ?,
	                  angeheftet = ?, gilt_bis = ?, updated_at = ? WHERE id = ?`,
		f.Betrifft, f.Text, f.Gewicht, f.Status, boolZahl(f.Angeheftet), f.GiltBis, now(), f.ID)
	if err != nil {
		return err
	}
	return d.indexFact(f.ID, f.Text, f.Betrifft)
}

// Zurueckziehen setzt einen Fakt außer Kraft, statt ihn zu löschen. So bleibt
// darstellbar, dass etwas einmal galt: "sie war Ärztin, bis sie kündigte".
func (d *DB) Zurueckziehen(id, abNode int64) error {
	_, err := d.Exec(`UPDATE memory_fact SET status = 'retired', gilt_bis = ?, updated_at = ? WHERE id = ?`,
		abNode, now(), id)
	return err
}

func (d *DB) DeleteFact(id int64) error {
	if _, err := d.Exec(`DELETE FROM memory_fts WHERE rowid = ?`, id); err != nil {
		return err
	}
	_, err := d.Exec(`DELETE FROM memory_fact WHERE id = ?`, id)
	return err
}
