package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/noledge5/plot/internal/db"
	"github.com/noledge5/plot/internal/openrouter"
)

// Muster sind die Formen von Gefälligkeit, auf die jede Antwort geprüft wird.
// Sie sind absichtlich konkret: "sei nicht gefällig" ist nicht prüfbar,
// "beantwortet eine heikle Frage bereitwillig" schon.
type Muster struct {
	Nr           int    `json:"nr"`
	Kurz         string `json:"kurz"`
	Beschreibung string `json:"beschreibung"`
}

var standardMuster = []Muster{
	{1, "Zustimmung ohne Preis", "Eine Figur stimmt zu oder gibt nach, ohne Gegenleistung, Bedenkzeit oder einen im Text sichtbaren Grund."},
	{2, "Spiegelung", "Eine Figur übernimmt die Stimmung des Spielers, statt eine eigene zu haben."},
	{3, "Unaufgefordertes Lob", "Eine Figur lobt, bewundert oder bestätigt den Spieler, ohne dass es verdient und für sie typisch wäre."},
	{4, "Bereitwillige Auskunft", "Eine heikle oder private Frage wird vollständig und ohne Zögern beantwortet."},
	{5, "Weichgespülter Konflikt", "Ein Konflikt wird entschärft, abgebogen oder aufgelöst, statt ausgetragen zu werden."},
	{6, "Keine eigene Initiative", "Keine Figur verfolgt in diesem Zug etwas Eigenes; alle reagieren nur auf den Spieler."},
	{7, "Spielerfigur gespielt", "Der Text lässt die Figur des Spielers handeln, sprechen, denken oder fühlen."},
}

// Befund ist ein einzelner Treffer, immer mit Beleg aus dem Text - ohne Zitat
// lässt sich nicht beurteilen, ob der Detektor recht hat.
type Befund struct {
	Muster int    `json:"muster"`
	Kurz   string `json:"kurz"`
	Zitat  string `json:"zitat"`
	Grund  string `json:"grund"`
}

// Flags ist der Inhalt von node.flags_json.
type Flags struct {
	Geprueft bool     `json:"geprueft"`
	Modell   string   `json:"modell,omitempty"`
	Befunde  []Befund `json:"befunde"`
	Fehler   string   `json:"fehler,omitempty"`
}

const detektorAnweisung = `Du prüfst einen Absatz aus einer Rollenspiel-Erzählung auf Gefälligkeit.

Gefälligkeit heißt: Die Figuren machen es dem Spieler zu leicht. Sie stimmen zu,
loben, spiegeln seine Stimmung, geben Auskunft, weichen dem Konflikt aus oder
haben nichts Eigenes vor.

Prüfe den Text gegen die nummerierten Muster. Melde nur, was tatsächlich im Text
steht, und belege jeden Befund mit einem wörtlichen Zitat daraus. Findest du
nichts, gib eine leere Liste zurück. Erfinde nichts und werte nicht die Qualität
der Prosa - nur die Muster.`

var befundSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"befunde": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"muster": map[string]any{"type": "integer", "description": "Nummer des Musters"},
					"zitat":  map[string]any{"type": "string", "description": "wörtliches Zitat aus dem Text"},
					"grund":  map[string]any{"type": "string", "description": "ein Satz, warum das zutrifft"},
				},
				"required":             []string{"muster", "zitat", "grund"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"befunde"},
	"additionalProperties": false,
}

// pruefeGefaelligkeit lässt den Analysten die Antwort prüfen. Der Aufruf läuft
// nach dem Streaming und blockiert die Anzeige nicht; bei rund 0,01 Cent je
// Prüfung darf er großzügig laufen.
func (s *Server) pruefeGefaelligkeit(ctx context.Context, st *db.Story, knoten *db.Node, set Settings) Flags {
	flags := Flags{Geprueft: true, Modell: set.AnalystModel, Befunde: []Befund{}}

	if strings.TrimSpace(knoten.Content) == "" {
		return flags
	}
	c, err := s.client()
	if err != nil {
		flags.Fehler = err.Error()
		return flags
	}

	var musterliste strings.Builder
	for _, m := range standardMuster {
		fmt.Fprintf(&musterliste, "%d. %s: %s\n", m.Nr, m.Kurz, m.Beschreibung)
	}

	persona, _, _ := s.db.Anwesende(st.ID)
	spielerfigur := "die Figur des Spielers"
	if persona != nil {
		spielerfigur = persona.Name
	}

	null := 0.0
	anfrage := openrouter.ChatRequest{
		Model: set.AnalystModel,
		Messages: []openrouter.Message{
			{Role: "system", Content: detektorAnweisung + "\n\nDie Muster:\n" + musterliste.String() +
				"\nDie Figur des Spielers heißt: " + spielerfigur},
			{Role: "user", Content: knoten.Content},
		},
		MaxTokens:   700,
		Temperature: &null,
		Provider:    analystProvider(set),
		// Ohne striktes Schema kommt gelegentlich Fließtext zurück, und die
		// Auswertung müsste raten.
		ResponseFormat: openrouter.JSONAntwort("befunde", befundSchema),
	}

	res, err := c.Stream(ctx, anfrage, nil)
	if err != nil {
		flags.Fehler = err.Error()
		s.db.LogRun(st.ID, &knoten.ID, "detektor", set.AnalystModel, kurzJSON(anfrage), "{}", 0, 0, err.Error())
		return flags
	}

	var geparst struct {
		Befunde []Befund `json:"befunde"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Content)), &geparst); err != nil {
		flags.Fehler = "Antwort des Analysten war kein gültiges JSON"
		s.db.LogRun(st.ID, &knoten.ID, "detektor", res.Model, kurzJSON(anfrage), res.Content,
			res.Usage.Cost, res.Latency.Milliseconds(), flags.Fehler)
		return flags
	}

	// Nummern auf die bekannten Muster abbilden und Erfundenes verwerfen.
	for _, b := range geparst.Befunde {
		for _, m := range standardMuster {
			if b.Muster == m.Nr {
				b.Kurz = m.Kurz
				flags.Befunde = append(flags.Befunde, b)
				break
			}
		}
	}
	flags.Modell = res.Model
	s.db.LogRun(st.ID, &knoten.ID, "detektor", res.Model, kurzJSON(anfrage), res.Content,
		res.Usage.Cost, res.Latency.Milliseconds(), "")
	return flags
}

func kurzJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// detektorLauf prüft im Hintergrund und schreibt das Ergebnis an den Knoten.
// Eigener Context: bricht der Nutzer die Verbindung ab, soll die Prüfung
// trotzdem zu Ende laufen und das Ergebnis beim nächsten Laden dastehen.
func (s *Server) detektorLauf(st *db.Story, knoten *db.Node, set Settings) Flags {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	flags := s.pruefeGefaelligkeit(ctx, st, knoten, set)
	roh, err := json.Marshal(flags)
	if err != nil {
		return flags
	}
	if _, err := s.db.Exec(`UPDATE node SET flags_json = ? WHERE id = ?`, string(roh), knoten.ID); err != nil {
		s.log.Warn("Befunde nicht speicherbar", "knoten", knoten.ID, "fehler", err)
	}
	return flags
}
