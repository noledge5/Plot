package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// FigurVorlage ist die Grundbeschreibung einer Figur, losgelöst von einer
// Geschichte. Wer dieselbe Person in mehreren Geschichten spielt, soll sie
// nicht jedes Mal neu beschreiben - eine zweite, leicht abweichende Fassung
// wäre eine andere Figur, auch wenn sie denselben Namen trägt.
//
// Geteilt wird nur das Blatt. Der Beziehungsstand entsteht aus den Deltas
// entlang eines Spielpfades und bleibt bei der Geschichte, in der er
// entstanden ist.
type FigurVorlage struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Rolle     string `json:"rolle"`
	Sheet     Sheet  `json:"sheet"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

const vorlageSpalten = `id, name, rolle, sheet_json, created_at, updated_at`

func scanVorlage(sc interface{ Scan(...any) error }) (*FigurVorlage, error) {
	var v FigurVorlage
	var roh string
	if err := sc.Scan(&v.ID, &v.Name, &v.Rolle, &roh, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(roh), &v.Sheet)
	return &v, nil
}

// MerkeFigur legt eine Figur in der Bibliothek ab. Gibt es sie unter diesem
// Namen und dieser Rolle schon, wird sie überschrieben: Zwei Fassungen
// derselben Person laufen mit der Zeit auseinander, und genau das soll die
// Bibliothek verhindern.
func (d *DB) MerkeFigur(name, rolle string, sheet Sheet) (*FigurVorlage, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("die Figur braucht einen Namen")
	}
	if rolle != "persona" {
		rolle = "npc"
	}
	roh, err := json.Marshal(sheet)
	if err != nil {
		return nil, err
	}
	ts := now()
	_, err = d.Exec(`INSERT INTO figur_vorlage (name, rolle, sheet_json, created_at, updated_at)
	                 VALUES (?, ?, ?, ?, ?)
	                 ON CONFLICT(name, rolle) DO UPDATE SET sheet_json = excluded.sheet_json,
	                                                        updated_at = excluded.updated_at`,
		name, rolle, string(roh), ts, ts)
	if err != nil {
		return nil, err
	}
	return d.FigurVorlageNach(name, rolle)
}

func (d *DB) FigurVorlageNach(name, rolle string) (*FigurVorlage, error) {
	v, err := scanVorlage(d.QueryRow(`SELECT `+vorlageSpalten+
		` FROM figur_vorlage WHERE name = ? AND rolle = ?`, name, rolle))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

func (d *DB) FigurVorlage(id int64) (*FigurVorlage, error) {
	v, err := scanVorlage(d.QueryRow(`SELECT `+vorlageSpalten+` FROM figur_vorlage WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

func (d *DB) Bibliothek() ([]FigurVorlage, error) {
	rows, err := d.Query(`SELECT ` + vorlageSpalten + ` FROM figur_vorlage ORDER BY rolle DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FigurVorlage{}
	for rows.Next() {
		v, err := scanVorlage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func (d *DB) VergissFigur(id int64) error {
	_, err := d.Exec(`DELETE FROM figur_vorlage WHERE id = ?`, id)
	return err
}
