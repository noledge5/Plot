package db

// Achsen sind die Richtungen, in denen sich ein Verhältnis bewegen kann.
// Gerichtet gedacht: was die Figur für die Spielerfigur empfindet.
var Achsen = []string{
	"trust", "warmth", "attraction", "respect",
	"familiarity", "tension", "resentment", "obligation", "fear",
}

// Delta ist eine einzelne Veränderung mit Begründung und Beleg. Ohne beides
// ließe sich später nicht beurteilen, ob sie berechtigt war.
type Delta struct {
	ID          int64  `json:"id"`
	StoryID     int64  `json:"storyId"`
	NodeID      int64  `json:"nodeId"`
	Figur       string `json:"figur"`
	Achse       string `json:"achse"`
	Delta       int    `json:"delta"`
	Begruendung string `json:"begruendung"`
	Zitat       string `json:"zitat"`
	CreatedAt   string `json:"createdAt"`
}

func (d *DB) AddDelta(dl Delta) error {
	_, err := d.Exec(`INSERT INTO rel_delta (story_id, node_id, figur, achse, delta, begruendung, zitat, created_at)
	                  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		dl.StoryID, dl.NodeID, dl.Figur, dl.Achse, dl.Delta, dl.Begruendung, dl.Zitat, now())
	return err
}

// Deltas liefert alle Veränderungen einer Geschichte, älteste zuerst. Welche
// davon gelten, entscheidet sich am Pfad - siehe Beziehungsstand.
func (d *DB) Deltas(storyID int64) ([]Delta, error) {
	rows, err := d.Query(`SELECT id, story_id, node_id, figur, achse, delta, begruendung, zitat, created_at
	                      FROM rel_delta WHERE story_id = ? ORDER BY id`, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Delta{}
	for rows.Next() {
		var dl Delta
		if err := rows.Scan(&dl.ID, &dl.StoryID, &dl.NodeID, &dl.Figur, &dl.Achse,
			&dl.Delta, &dl.Begruendung, &dl.Zitat, &dl.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, dl)
	}
	return out, rows.Err()
}

func (d *DB) DeleteDelta(id int64) error {
	_, err := d.Exec(`DELETE FROM rel_delta WHERE id = ?`, id)
	return err
}

// Beziehungsstand faltet die Ausgangswerte einer Figur mit allen Deltas, die
// auf dieser Zeitlinie liegen. Nicht gespeichert, sondern gerechnet: dadurch
// kann ein verworfener Zweig keinen Zustand hinterlassen.
func Beziehungsstand(start map[string]int, deltas []Delta, figur string, aufPfad map[int64]bool) map[string]int {
	stand := map[string]int{}
	for _, a := range Achsen {
		stand[a] = start[a]
	}
	for _, dl := range deltas {
		if dl.Figur != figur || !aufPfad[dl.NodeID] {
			continue
		}
		if _, bekannt := stand[dl.Achse]; !bekannt {
			continue
		}
		stand[dl.Achse] = begrenzen(stand[dl.Achse] + dl.Delta)
	}
	return stand
}

func begrenzen(v int) int {
	if v > 100 {
		return 100
	}
	if v < -100 {
		return -100
	}
	return v
}
