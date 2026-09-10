package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("nicht gefunden")

func now() string { return time.Now().UTC().Format(time.RFC3339) }

type Story struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	SystemPrompt string `json:"systemPrompt"`
	SettingsJSON string `json:"-"`
	HeadNodeID   *int64 `json:"headNodeId"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type Node struct {
	ID        int64  `json:"id"`
	StoryID   int64  `json:"storyId"`
	ParentID  *int64 `json:"parentId"`
	Kind      string `json:"kind"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Model     string `json:"model"`
	UsageJSON string `json:"usage"`
	FlagsJSON string `json:"flags"`
	CreatedAt string `json:"createdAt"`
	// Siblings sagt der Oberfläche, ob es zu diesem Turn Alternativen gibt
	// (aus Regenerieren oder aus dem Modellvergleich).
	Siblings int `json:"siblings"`
}

type Slot struct {
	ID        int64  `json:"id"`
	StoryID   int64  `json:"storyId"`
	NodeID    int64  `json:"nodeId"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Preview   string `json:"preview"`
	CreatedAt string `json:"createdAt"`
}

// ---------- Stories ----------

func (d *DB) CreateStory(title, systemPrompt string) (*Story, error) {
	ts := now()
	res, err := d.Exec(`INSERT INTO story (title, system_prompt, created_at, updated_at)
	                    VALUES (?, ?, ?, ?)`, title, systemPrompt, ts, ts)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return d.Story(id)
}

func (d *DB) Story(id int64) (*Story, error) {
	var s Story
	err := d.QueryRow(`SELECT id, title, system_prompt, settings_json, head_node_id, created_at, updated_at
	                   FROM story WHERE id = ?`, id).
		Scan(&s.ID, &s.Title, &s.SystemPrompt, &s.SettingsJSON, &s.HeadNodeID, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func (d *DB) Stories() ([]Story, error) {
	rows, err := d.Query(`SELECT id, title, system_prompt, settings_json, head_node_id, created_at, updated_at
	                      FROM story ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Story{}
	for rows.Next() {
		var s Story
		if err := rows.Scan(&s.ID, &s.Title, &s.SystemPrompt, &s.SettingsJSON, &s.HeadNodeID,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) UpdateStory(id int64, title, systemPrompt, settingsJSON string) error {
	_, err := d.Exec(`UPDATE story SET title = ?, system_prompt = ?, settings_json = ?, updated_at = ?
	                  WHERE id = ?`, title, systemPrompt, settingsJSON, now(), id)
	return err
}

func (d *DB) DeleteStory(id int64) error {
	_, err := d.Exec(`DELETE FROM story WHERE id = ?`, id)
	return err
}

// SetHead setzt die aktuelle Position im Baum. Das ist alles, was "einen
// Speicherstand laden" bedeutet - es wird nichts kopiert und nichts gelöscht.
func (d *DB) SetHead(storyID int64, nodeID *int64) error {
	_, err := d.Exec(`UPDATE story SET head_node_id = ?, updated_at = ? WHERE id = ?`,
		nodeID, now(), storyID)
	return err
}

// ---------- Knoten ----------

func (d *DB) AddNode(storyID int64, parentID *int64, kind, role, content, model, usageJSON string) (*Node, error) {
	if usageJSON == "" {
		usageJSON = "{}"
	}
	res, err := d.Exec(`INSERT INTO node (story_id, parent_id, kind, role, content, model, usage_json, created_at)
	                    VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		storyID, parentID, kind, role, content, model, usageJSON, now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if err := d.SetHead(storyID, &id); err != nil {
		return nil, err
	}
	return d.Node(id)
}

func (d *DB) Node(id int64) (*Node, error) {
	var n Node
	err := d.QueryRow(`SELECT id, story_id, parent_id, kind, role, content, model, usage_json, flags_json, created_at
	                   FROM node WHERE id = ?`, id).
		Scan(&n.ID, &n.StoryID, &n.ParentID, &n.Kind, &n.Role, &n.Content, &n.Model,
			&n.UsageJSON, &n.FlagsJSON, &n.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &n, err
}

func (d *DB) UpdateNodeContent(id int64, content string) error {
	_, err := d.Exec(`UPDATE node SET content = ? WHERE id = ?`, content, id)
	return err
}

func (d *DB) SetNodeUsage(id int64, model, usageJSON string) error {
	_, err := d.Exec(`UPDATE node SET model = ?, usage_json = ? WHERE id = ?`, model, usageJSON, id)
	return err
}

// Path liefert den Weg von der Wurzel bis zum Knoten - die eine Zeitlinie, die
// gerade gilt. Alles, was in anderen Zweigen steht, ist hier unsichtbar; genau
// deshalb kann ein Speicherstand keinen Zustand aus einer verworfenen Fassung
// sehen.
func (d *DB) Path(nodeID int64) ([]Node, error) {
	rows, err := d.Query(`
WITH RECURSIVE pfad(id, story_id, parent_id, kind, role, content, model, usage_json, flags_json, created_at, tiefe) AS (
	SELECT id, story_id, parent_id, kind, role, content, model, usage_json, flags_json, created_at, 0
	FROM node WHERE id = ?
	UNION ALL
	SELECT n.id, n.story_id, n.parent_id, n.kind, n.role, n.content, n.model, n.usage_json, n.flags_json, n.created_at, pfad.tiefe + 1
	FROM node n JOIN pfad ON n.id = pfad.parent_id
)
SELECT p.id, p.story_id, p.parent_id, p.kind, p.role, p.content, p.model, p.usage_json, p.flags_json, p.created_at,
       (SELECT count(*) FROM node g WHERE g.story_id = p.story_id
          AND (g.parent_id IS p.parent_id)) AS geschwister
FROM pfad p ORDER BY p.tiefe DESC`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Node{}
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.StoryID, &n.ParentID, &n.Kind, &n.Role, &n.Content, &n.Model,
			&n.UsageJSON, &n.FlagsJSON, &n.CreatedAt, &n.Siblings); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Alternatives liefert die Geschwister eines Knotens, also die anderen
// Fassungen desselben Zuges - aus Regenerieren oder aus dem Modellvergleich.
func (d *DB) Alternatives(nodeID int64) ([]Node, error) {
	n, err := d.Node(nodeID)
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	q := `SELECT id, story_id, parent_id, kind, role, content, model, usage_json, flags_json, created_at
	      FROM node WHERE story_id = ? AND parent_id IS ? ORDER BY id`
	rows, err = d.Query(q, n.StoryID, n.ParentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Node{}
	for rows.Next() {
		var a Node
		if err := rows.Scan(&a.ID, &a.StoryID, &a.ParentID, &a.Kind, &a.Role, &a.Content, &a.Model,
			&a.UsageJSON, &a.FlagsJSON, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---------- Speicherstände ----------

func (d *DB) CreateSlot(storyID, nodeID int64, name, kind, preview string) (*Slot, error) {
	res, err := d.Exec(`INSERT INTO save_slot (story_id, node_id, name, kind, preview, created_at)
	                    VALUES (?, ?, ?, ?, ?, ?)`, storyID, nodeID, name, kind, preview, now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	var s Slot
	err = d.QueryRow(`SELECT id, story_id, node_id, name, kind, preview, created_at FROM save_slot WHERE id = ?`, id).
		Scan(&s.ID, &s.StoryID, &s.NodeID, &s.Name, &s.Kind, &s.Preview, &s.CreatedAt)
	return &s, err
}

func (d *DB) Slots(storyID int64) ([]Slot, error) {
	rows, err := d.Query(`SELECT id, story_id, node_id, name, kind, preview, created_at
	                      FROM save_slot WHERE story_id = ? ORDER BY created_at DESC`, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Slot{}
	for rows.Next() {
		var s Slot
		if err := rows.Scan(&s.ID, &s.StoryID, &s.NodeID, &s.Name, &s.Kind, &s.Preview, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) DeleteSlot(id int64) error {
	_, err := d.Exec(`DELETE FROM save_slot WHERE id = ?`, id)
	return err
}

// PruneAutoSlots hält die automatischen Speicherstände auf einer Höchstzahl.
func (d *DB) PruneAutoSlots(storyID int64, keep int) error {
	_, err := d.Exec(`DELETE FROM save_slot WHERE story_id = ? AND kind = 'auto' AND id NOT IN (
		SELECT id FROM save_slot WHERE story_id = ? AND kind = 'auto' ORDER BY id DESC LIMIT ?)`,
		storyID, storyID, keep)
	return err
}

// ---------- Protokoll ----------

type RunLog struct {
	ID        int64   `json:"id"`
	NodeID    *int64  `json:"nodeId"`
	Purpose   string  `json:"purpose"`
	Model     string  `json:"model"`
	Request   string  `json:"request"`
	Response  string  `json:"response"`
	CostUSD   float64 `json:"costUsd"`
	LatencyMS int64   `json:"latencyMs"`
	Error     string  `json:"error"`
	CreatedAt string  `json:"createdAt"`
}

func (d *DB) LogRun(storyID int64, nodeID *int64, purpose, model, request, response string, cost float64, latencyMS int64, errMsg string) (int64, error) {
	res, err := d.Exec(`INSERT INTO run_log (story_id, node_id, purpose, model, request_json, response_json, cost_usd, latency_ms, error, created_at)
	                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		storyID, nodeID, purpose, model, request, response, cost, latencyMS, errMsg, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RunForNode liefert den Erzählaufruf zu einem Knoten - die Grundlage des
// Payload-Inspektors. Ausdrücklich nach Zweck gefiltert: an einem Knoten
// hängen auch Auswertungsaufrufe, und der jüngste ist selten der gesuchte.
func (d *DB) RunForNode(nodeID int64) (*RunLog, error) {
	var r RunLog
	err := d.QueryRow(`SELECT id, node_id, purpose, model, request_json, response_json, cost_usd, latency_ms, error, created_at
	                   FROM run_log WHERE node_id = ? AND purpose = 'narrate'
	                   ORDER BY id DESC LIMIT 1`, nodeID).
		Scan(&r.ID, &r.NodeID, &r.Purpose, &r.Model, &r.Request, &r.Response, &r.CostUSD, &r.LatencyMS, &r.Error, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &r, err
}

// RunsForNode liefert alle Aufrufe zu einem Knoten, jüngste zuerst - für den
// Inspektor, der neben der Erzählung auch die Prüfung zeigen soll.
func (d *DB) RunsForNode(nodeID int64) ([]RunLog, error) {
	rows, err := d.Query(`SELECT id, node_id, purpose, model, request_json, response_json, cost_usd, latency_ms, error, created_at
	                      FROM run_log WHERE node_id = ? ORDER BY id DESC`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RunLog{}
	for rows.Next() {
		var r RunLog
		if err := rows.Scan(&r.ID, &r.NodeID, &r.Purpose, &r.Model, &r.Request, &r.Response,
			&r.CostUSD, &r.LatencyMS, &r.Error, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Costs summiert die Kosten einer Story - für die Anzeige "was hat diese
// Geschichte bisher gekostet".
func (d *DB) Costs(storyID int64) (total float64, calls int, err error) {
	err = d.QueryRow(`SELECT COALESCE(SUM(cost_usd), 0), count(*) FROM run_log WHERE story_id = ?`, storyID).
		Scan(&total, &calls)
	return
}

func (d *DB) SetSlotNode(slotID, nodeID int64) error {
	_, err := d.Exec(`UPDATE save_slot SET node_id = ? WHERE id = ?`, nodeID, slotID)
	if err != nil {
		return fmt.Errorf("speicherstand verschieben: %w", err)
	}
	return nil
}
