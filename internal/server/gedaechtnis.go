package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/noledge5/plot/internal/db"
	"github.com/noledge5/plot/internal/openrouter"
)

// Wie viele Züge am Stück zusammengefasst werden, sobald sie aus dem
// wörtlichen Verlauf fallen.
const chronikAbschnitt = 8

// --- Chronik ---

const chronikAnweisung = `Du fasst einen Abschnitt einer Rollenspiel-Erzählung zusammen.

Schreib in der Vergangenheitsform, dritte Person, höchstens sechs Sätze.
Halte fest: was geschah, wer beteiligt war, was sich zwischen den Figuren
verändert hat und was offen blieb. Nenne Namen statt "er" und "sie".

Keine Deutung, keine Bewertung, keine Ausschmückung - nur was im Text steht.`

// chronikNachziehen fasst zusammen, was gerade aus dem wörtlichen Verlauf
// gefallen ist. Ohne das verschwindet der Anfang einer Geschichte einfach,
// sobald der Deckel greift.
func (s *Server) chronikNachziehen(ctx context.Context, st *db.Story, pfad []db.Node, set Settings) {
	if len(pfad) <= set.MaxHistoryTurns {
		return // noch nichts verloren
	}
	bisher, err := s.db.LetzteZusammengefasst(st.ID)
	if err != nil {
		return
	}

	// Alles, was außerhalb des wörtlichen Verlaufs liegt und noch nicht in
	// der Chronik steht.
	aus := pfad[:len(pfad)-set.MaxHistoryTurns]
	var offen []db.Node
	for _, n := range aus {
		if n.ID > bisher {
			offen = append(offen, n)
		}
	}
	if len(offen) < chronikAbschnitt {
		return // lohnt noch keinen Aufruf
	}

	c, err := s.client()
	if err != nil {
		return
	}
	var text strings.Builder
	for _, n := range offen {
		wer := "Spieler"
		if n.Role == "assistant" {
			wer = "Erzähler"
		}
		if n.Kind == KindRegie {
			continue
		}
		inhalt := n.Content
		if n.Kind == KindDialog {
			inhalt = alsRede(inhalt)
		}
		fmt.Fprintf(&text, "[%s] %s\n\n", wer, inhalt)
	}

	null := 0.0
	res, err := c.Stream(ctx, openrouter.ChatRequest{
		Model:       set.AnalystModel,
		Messages:    []openrouter.Message{{Role: "system", Content: chronikAnweisung}, {Role: "user", Content: text.String()}},
		MaxTokens:   500,
		Temperature: &null,
		Provider:    analystProvider(set),
	}, nil)
	if err != nil {
		s.db.LogRun(st.ID, nil, "chronik", set.AnalystModel, "{}", "{}", 0, 0, err.Error())
		return
	}
	zusammen := strings.TrimSpace(res.Content)
	if zusammen == "" {
		return
	}
	if _, err := s.db.CreateSummary(st.ID, 1, offen[0].ID, offen[len(offen)-1].ID, zusammen); err != nil {
		s.log.Warn("Chronik nicht speicherbar", "fehler", err)
		return
	}
	s.db.LogRun(st.ID, nil, "chronik", res.Model, "{}", zusammen, res.Usage.Cost, res.Latency.Milliseconds(), "")
}

// --- Fakten ---

const faktenAnweisung = `Du hältst fest, was aus einem Abschnitt einer Rollenspiel-Erzählung dauerhaft gilt.

Nimm nur auf, was später noch wichtig sein kann: Eigenschaften und Verhältnisse
von Figuren, Abmachungen, Orte, Gegenstände, Ereignisse mit Folgen.

Nicht aufnehmen: was gerade nur geschieht (Bewegungen, Gesten, Stimmungen),
Vermutungen des Erzählers, und alles, was nicht ausdrücklich im Text steht.

Jeder Eintrag ist ein einzelner, für sich verständlicher Satz mit Namen statt
Fürwörtern. Gewicht 80-100 für Dinge, die die Geschichte tragen, 30-50 für
Beiläufiges. Findest du nichts Dauerhaftes, gib eine leere Liste zurück.`

var faktenSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"fakten": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"betrifft": map[string]any{"type": "string", "description": "Name der Figur oder des Ortes"},
					"text":     map[string]any{"type": "string", "description": "ein Satz, für sich verständlich"},
					"gewicht":  map[string]any{"type": "integer", "description": "0-100"},
				},
				"required":             []string{"betrifft", "text", "gewicht"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"fakten"},
	"additionalProperties": false,
}

// faktenGewinnen liest aus einer Antwort heraus, was dauerhaft gilt, und legt
// es ab. Läuft nach dem Streaming; bei den Preisen des Analysten kostet das
// ungefähr so viel wie ein Zeichen Prosa.
func (s *Server) faktenGewinnen(ctx context.Context, st *db.Story, knoten *db.Node, set Settings) int {
	if strings.TrimSpace(knoten.Content) == "" {
		return 0
	}
	c, err := s.client()
	if err != nil {
		return 0
	}

	null := 0.0
	res, err := c.Stream(ctx, openrouter.ChatRequest{
		Model: set.AnalystModel,
		Messages: []openrouter.Message{
			{Role: "system", Content: faktenAnweisung},
			{Role: "user", Content: knoten.Content},
		},
		MaxTokens:      800,
		Temperature:    &null,
		Provider:       analystProvider(set),
		ResponseFormat: openrouter.JSONAntwort("fakten", faktenSchema),
	}, nil)
	if err != nil {
		s.db.LogRun(st.ID, &knoten.ID, "fakten", set.AnalystModel, "{}", "{}", 0, 0, err.Error())
		return 0
	}

	var geparst struct {
		Fakten []struct {
			Betrifft string `json:"betrifft"`
			Text     string `json:"text"`
			Gewicht  int    `json:"gewicht"`
		} `json:"fakten"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Content)), &geparst); err != nil {
		s.db.LogRun(st.ID, &knoten.ID, "fakten", res.Model, "{}", res.Content,
			res.Usage.Cost, res.Latency.Milliseconds(), "Antwort war kein gültiges JSON")
		return 0
	}

	vorhanden, _ := s.db.Facts(st.ID, "")
	neu := 0
	for _, f := range geparst.Fakten {
		text := strings.TrimSpace(f.Text)
		if text == "" {
			continue
		}
		if istDoppelt(text, vorhanden) {
			continue
		}
		status := "canon"
		if !set.FaktenAutomatisch {
			status = "proposed"
		}
		angelegt, err := s.db.CreateFact(db.Fact{
			StoryID: st.ID, Betrifft: strings.TrimSpace(f.Betrifft), Text: text,
			Gewicht: f.Gewicht, Status: status, Quelle: &knoten.ID, GiltAb: &knoten.ID,
		})
		if err != nil {
			continue
		}
		vorhanden = append(vorhanden, *angelegt)
		neu++
	}
	s.db.LogRun(st.ID, &knoten.ID, "fakten", res.Model, "{}", res.Content,
		res.Usage.Cost, res.Latency.Milliseconds(), "")
	return neu
}

// istDoppelt hält denselben Sachverhalt davon ab, bei jedem Zug erneut
// aufgeschrieben zu werden. Kein Vektorvergleich, sondern Wortüberlappung -
// das reicht für Sätze, die ohnehin aus demselben Text stammen.
func istDoppelt(text string, vorhanden []db.Fact) bool {
	neu := wortmenge(text)
	if len(neu) == 0 {
		return false
	}
	for _, f := range vorhanden {
		alt := wortmenge(f.Text)
		if len(alt) == 0 {
			continue
		}
		gleich := 0
		for w := range neu {
			if alt[w] {
				gleich++
			}
		}
		kleiner := len(neu)
		if len(alt) < kleiner {
			kleiner = len(alt)
		}
		if float64(gleich)/float64(kleiner) > 0.75 {
			return true
		}
	}
	return false
}

func wortmenge(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,;:!?\"'()„“—-")
		if len([]rune(w)) >= 4 {
			out[w] = true
		}
	}
	return out
}

// --- Abruf ---

// gueltig prüft, ob ein Fakt auf dieser Zeitlinie gilt. Ein Fakt aus einem
// verworfenen Zweig darf hier nicht auftauchen - das ist der Grund, warum
// Herkunft und Gültigkeit am Knoten hängen.
func gueltig(f db.Fact, aufPfad map[int64]bool) bool {
	if f.Status == "retired" || f.Status == "proposed" {
		return false
	}
	if f.GiltAb != nil && !aufPfad[*f.GiltAb] {
		return false
	}
	if f.GiltBis != nil && aufPfad[*f.GiltBis] {
		return false
	}
	return true
}

// abrufFakten wählt aus, was in den Prompt kommt. Immer dabei: angeheftete
// Fakten und solche über anwesende Figuren. Dazu, was zur letzten Eingabe
// passt - denn danach wird gleich gefragt.
func (s *Server) abrufFakten(st *db.Story, pfad []db.Node, npcs []db.Character, grenze int) []db.Fact {
	if grenze <= 0 {
		grenze = 12
	}
	aufPfad := map[int64]bool{}
	for _, n := range pfad {
		aufPfad[n.ID] = true
	}
	anwesend := map[string]bool{}
	for _, c := range npcs {
		anwesend[strings.ToLower(c.Name)] = true
	}

	// Die letzten Eingaben sind der beste Hinweis darauf, worum es gleich geht.
	var suchtext strings.Builder
	for i := len(pfad) - 1; i >= 0 && i > len(pfad)-4; i-- {
		suchtext.WriteString(pfad[i].Content + " ")
	}

	punkte := map[int64]float64{}
	kandidaten := map[int64]db.Fact{}

	alle, err := s.db.Facts(st.ID, "canon")
	if err != nil {
		return nil
	}
	for _, f := range alle {
		if !gueltig(f, aufPfad) {
			continue
		}
		p := float64(f.Gewicht) / 100
		if f.Angeheftet {
			p += 10 // angeheftet heißt: gilt immer
		}
		if anwesend[strings.ToLower(f.Betrifft)] {
			p += 1.5 // wer im Raum steht, ist wichtiger als wer nicht da ist
		}
		punkte[f.ID] = p
		kandidaten[f.ID] = f
	}

	if treffer, err := s.db.SucheFakten(st.ID, suchtext.String(), 20); err == nil {
		for _, f := range treffer {
			if _, dabei := kandidaten[f.ID]; !dabei {
				if !gueltig(f, aufPfad) {
					continue
				}
				kandidaten[f.ID] = f
				punkte[f.ID] = float64(f.Gewicht) / 100
			}
			punkte[f.ID] += 2 // passt zum, was gerade gesagt wurde
		}
	}

	sortiert := make([]db.Fact, 0, len(kandidaten))
	for _, f := range kandidaten {
		sortiert = append(sortiert, f)
	}
	sort.Slice(sortiert, func(i, j int) bool {
		if punkte[sortiert[i].ID] != punkte[sortiert[j].ID] {
			return punkte[sortiert[i].ID] > punkte[sortiert[j].ID]
		}
		return sortiert[i].ID > sortiert[j].ID
	})
	if len(sortiert) > grenze {
		sortiert = sortiert[:grenze]
	}
	return sortiert
}

func rendereFakten(fakten []db.Fact) string {
	if len(fakten) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range fakten {
		if f.Betrifft != "" {
			fmt.Fprintf(&b, "- (%s) %s\n", f.Betrifft, f.Text)
		} else {
			fmt.Fprintf(&b, "- %s\n", f.Text)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// rendereChronik gibt die Zusammenfassungen aus, die auf dieser Zeitlinie
// liegen - älteste zuerst.
func (s *Server) rendereChronik(storyID int64, pfad []db.Node) string {
	alle, err := s.db.Summaries(storyID)
	if err != nil || len(alle) == 0 {
		return ""
	}
	aufPfad := map[int64]bool{}
	for _, n := range pfad {
		aufPfad[n.ID] = true
	}
	var b strings.Builder
	for _, z := range alle {
		if !aufPfad[z.BisNode] {
			continue // gehört zu einem anderen Zweig
		}
		b.WriteString(strings.TrimSpace(z.Text) + "\n\n")
	}
	return strings.TrimSpace(b.String())
}

// gedaechtnisNachziehen läuft nach einem Zug: Fakten gewinnen und bei Bedarf
// die Chronik verlängern. Eigener Context, damit ein Verbindungsabbruch die
// Arbeit nicht abschneidet.
func (s *Server) gedaechtnisNachziehen(st *db.Story, knoten *db.Node, set Settings) int {
	if !set.GedaechtnisAn || strings.TrimSpace(set.AnalystModel) == "" {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	neu := s.faktenGewinnen(ctx, st, knoten, set)
	if pfad, err := s.db.Path(knoten.ID); err == nil {
		s.chronikNachziehen(ctx, st, pfad, set)
	}
	return neu
}
