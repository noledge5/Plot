package db

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// Sheet ist das Charakterblatt. Die Felder, auf die sich die Autonomie-Schicht
// stützt, sind eigene Felder und keine Prosa: Fließtext im Blatt wird vom
// Modell überlesen, eine Liste nicht.
type Sheet struct {
	Kern        string `json:"kern"`        // wer sie ist, in zwei, drei Sätzen
	Sprechweise string `json:"sprechweise"` // Satzbau, Wortwahl, Eigenheiten

	// Verhaeltnis beschreibt in Prosa, wie diese Figur zur Spielerfigur steht,
	// bevor irgendetwas geschieht. Ohne das spielt ein Modell die
	// Reibungsregeln als Grundton und begrüßt alte Freunde wie Fremde.
	Verhaeltnis string `json:"verhaeltnis"`
	// Beziehung sind die Ausgangswerte der Achsen, -100 bis 100. Was hier
	// nicht steht, startet bei 0 - also neutral, nicht ablehnend.
	Beziehung map[string]int `json:"beziehung"`

	// Drives sind das, was eine Figur von sich aus will. Ohne sie bleiben
	// Figuren höflich abwartend, egal was im Prompt steht.
	Drives []Drive `json:"drives"`

	// HardLimits gelten bei jedem Beziehungswert, ausnahmslos.
	HardLimits []string `json:"hardLimits"`
	// SoftLimits geben erst ab einer Schwelle nach.
	SoftLimits []SoftLimit `json:"softLimits"`
	// DealBreakers senken Werte drastisch, wenn sie eintreten.
	DealBreakers []string `json:"dealBreakers"`
	// Secrets werden erst ab einer Schwelle preisgegeben.
	Secrets []Secret `json:"secrets"`

	// Volatility skaliert, wie schnell sich eine Achse bei dieser Figur bewegt.
	// Unter 1 heißt: langsamer. Ab M4 in Gebrauch.
	Volatility map[string]float64 `json:"volatility"`
}

type Drive struct {
	Ziel string `json:"ziel"`
	// Druck steigt, solange das Ziel nicht vorankommt. Über der Schwelle
	// verfolgt die Figur es aktiv, auch wenn es gerade unpassend ist.
	Druck int `json:"druck"`
	// Sichtbar: darf der Spieler ahnen, worauf sie hinauswill?
	Sichtbar bool `json:"sichtbar"`
}

type SoftLimit struct {
	Was    string         `json:"was"`
	ErstAb map[string]int `json:"erstAb"` // Achse -> Mindestwert
}

type Secret struct {
	Text        string         `json:"text"`
	PreisgabeAb map[string]int `json:"preisgabeAb"`
}

type Character struct {
	ID         int64  `json:"id"`
	StoryID    int64  `json:"storyId"`
	Rolle      string `json:"rolle"` // npc | persona
	Name       string `json:"name"`
	Sheet      Sheet  `json:"sheet"`
	Aktiv      bool   `json:"aktiv"`
	Sortierung int    `json:"sortierung"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

func (d *DB) CreateCharacter(storyID int64, rolle, name string, sheet Sheet) (*Character, error) {
	roh, err := json.Marshal(sheet)
	if err != nil {
		return nil, err
	}
	ts := now()
	res, err := d.Exec(`INSERT INTO character (story_id, rolle, name, sheet_json, created_at, updated_at)
	                    VALUES (?, ?, ?, ?, ?, ?)`, storyID, rolle, name, string(roh), ts, ts)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return d.Character(id)
}

func scanCharacter(sc interface{ Scan(...any) error }) (*Character, error) {
	var c Character
	var roh string
	var aktiv int
	if err := sc.Scan(&c.ID, &c.StoryID, &c.Rolle, &c.Name, &roh, &aktiv,
		&c.Sortierung, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	c.Aktiv = aktiv != 0
	// Ein unlesbares Blatt darf die Figur nicht unsichtbar machen; sie kommt
	// dann eben mit leeren Feldern zurück und lässt sich neu ausfüllen.
	_ = json.Unmarshal([]byte(roh), &c.Sheet)
	return &c, nil
}

const characterSpalten = `id, story_id, rolle, name, sheet_json, aktiv, sortierung, created_at, updated_at`

func (d *DB) Character(id int64) (*Character, error) {
	c, err := scanCharacter(d.QueryRow(`SELECT `+characterSpalten+` FROM character WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (d *DB) Characters(storyID int64) ([]Character, error) {
	rows, err := d.Query(`SELECT `+characterSpalten+` FROM character WHERE story_id = ?
	                      ORDER BY rolle DESC, sortierung, id`, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Character{}
	for rows.Next() {
		c, err := scanCharacter(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// Anwesende liefert die Figuren, die gerade in der Szene stehen - nur sie
// gehören in den Prompt.
func (d *DB) Anwesende(storyID int64) (persona *Character, npcs []Character, err error) {
	alle, err := d.Characters(storyID)
	if err != nil {
		return nil, nil, err
	}
	for i := range alle {
		c := alle[i]
		if c.Rolle == "persona" {
			if persona == nil {
				persona = &c
			}
			continue
		}
		if c.Aktiv {
			npcs = append(npcs, c)
		}
	}
	return persona, npcs, nil
}

func (d *DB) UpdateCharacter(id int64, name string, sheet Sheet, aktiv bool, sortierung int) error {
	roh, err := json.Marshal(sheet)
	if err != nil {
		return err
	}
	an := 0
	if aktiv {
		an = 1
	}
	_, err = d.Exec(`UPDATE character SET name = ?, sheet_json = ?, aktiv = ?, sortierung = ?, updated_at = ?
	                 WHERE id = ?`, name, string(roh), an, sortierung, now(), id)
	return err
}

func (d *DB) DeleteCharacter(id int64) error {
	_, err := d.Exec(`DELETE FROM character WHERE id = ?`, id)
	return err
}
