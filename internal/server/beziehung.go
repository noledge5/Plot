package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/noledge5/plot/internal/db"
	"github.com/noledge5/plot/internal/openrouter"
)

// deckel begrenzt, wie weit sich eine Achse in einem einzigen Zug bewegen
// darf. Ohne das springen Modelle nach einem freundlichen Satz von Misstrauen
// auf Zuneigung - und nach einem schroffen zurück.
const deckel = 5

// beziehungsSaetze übersetzt einen Wert in Verhalten. Beide Richtungen sind
// besetzt: Ein Regelwerk, das nur beschreibt, wann eine Figur mauert, macht
// aus jeder Figur eine abweisende - auch aus einer befreundeten.
var beziehungsSaetze = map[string]struct{ Hoch, Niedrig string }{
	"trust": {
		Hoch:    "%s vertraut dir. Sie redet offen, auch über Dinge, die sie anderen nicht sagt.",
		Niedrig: "%s misstraut dir. Sie weicht persönlichen Fragen aus und prüft, ob deine Aussagen zu dem passen, was sie schon weiß.",
	},
	"warmth": {
		Hoch:    "%s mag dich. Sie freut sich, dich zu sehen, und zeigt es auf ihre Art.",
		Niedrig: "%s kann dich nicht leiden. Sie hält das Nötigste kurz.",
	},
	"attraction": {
		Hoch:    "%s fühlt sich zu dir hingezogen und kann es nicht ganz verbergen.",
		Niedrig: "%s findet dich abstoßend und hält körperlich Abstand.",
	},
	"respect": {
		Hoch:    "%s achtet dich. Sie nimmt ernst, was du sagst, auch wenn sie anderer Meinung ist.",
		Niedrig: "%s hält wenig von dir. Sie nimmt deine Einwände nicht zum Anlass, etwas zu ändern.",
	},
	"familiarity": {
		Hoch:    "Ihr kennt euch gut. %s braucht keine Höflichkeitsformeln und kommt gleich zur Sache.",
		Niedrig: "Ihr seid einander fremd. %s bleibt förmlich und vorsichtig.",
	},
	"tension": {
		Hoch:    "Zwischen euch steht etwas Ungeklärtes. %s spricht es nicht an, aber es färbt jedes Gespräch.",
		Niedrig: "",
	},
	"resentment": {
		Hoch:    "%s nimmt dir etwas übel und lässt es bei Gelegenheit durchblicken.",
		Niedrig: "",
	},
	"obligation": {
		Hoch:    "%s steht in deiner Schuld und weiß das.",
		Niedrig: "%s findet, dass du ihr etwas schuldest.",
	},
	"fear": {
		Hoch:    "%s fürchtet dich. Sie wägt jedes Wort ab und vermeidet Widerspruch.",
		Niedrig: "",
	},
}

// merklich ist die Schwelle, ab der eine Achse überhaupt erwähnt wird. Alles
// darunter ist Rauschen und würde den Prompt mit Widersprüchen füllen.
const merklich = 25

func rendereBeziehung(name string, stand map[string]int) string {
	achsen := make([]string, 0, len(stand))
	for a := range stand {
		achsen = append(achsen, a)
	}
	sort.Strings(achsen)

	var zeilen []string
	for _, a := range achsen {
		wert := stand[a]
		if wert > -merklich && wert < merklich {
			continue
		}
		vorlage, bekannt := beziehungsSaetze[a]
		if !bekannt {
			continue
		}
		satz := vorlage.Hoch
		if wert < 0 {
			satz = vorlage.Niedrig
		}
		if satz == "" {
			continue
		}
		zeilen = append(zeilen, fmt.Sprintf(satz, name))
	}
	return strings.Join(zeilen, "\n")
}

// rendereBeziehungen baut den Block "Wie sie zu dir stehen". Er kommt vor den
// Verhaltensregeln in den Prompt, damit er den Ton setzt und die Regeln ihn
// nur abwandeln.
func (s *Server) rendereBeziehungen(st *db.Story, pfad []db.Node, npcs []db.Character) string {
	if len(npcs) == 0 {
		return ""
	}
	deltas, err := s.db.Deltas(st.ID)
	if err != nil {
		deltas = nil
	}
	aufPfad := map[int64]bool{}
	for _, n := range pfad {
		aufPfad[n.ID] = true
	}

	var b strings.Builder
	for _, c := range npcs {
		if v := strings.TrimSpace(c.Sheet.Verhaeltnis); v != "" {
			b.WriteString(v + "\n")
		}
		stand := db.Beziehungsstand(c.Sheet.Beziehung, deltas, c.Name, aufPfad)
		if zeilen := rendereBeziehung(c.Name, stand); zeilen != "" {
			b.WriteString(zeilen + "\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// staende liefert den aktuellen Stand je Figur - für die Oberfläche und für
// den Detektor, der ohne diesen Hintergrund Freundlichkeit unter Vertrauten
// als Gefälligkeit meldet.
func (s *Server) staende(st *db.Story, pfad []db.Node, npcs []db.Character) map[string]map[string]int {
	deltas, _ := s.db.Deltas(st.ID)
	aufPfad := map[int64]bool{}
	for _, n := range pfad {
		aufPfad[n.ID] = true
	}
	out := map[string]map[string]int{}
	for _, c := range npcs {
		out[c.Name] = db.Beziehungsstand(c.Sheet.Beziehung, deltas, c.Name, aufPfad)
	}
	return out
}

// --- Fortschreibung ---

const beziehungAnweisung = `Du beurteilst, wie sich ein Abschnitt einer Rollenspiel-Erzählung auf die
Verhältnisse zwischen den Figuren und dem Spieler ausgewirkt hat.

Melde nur Veränderungen, für die es im Text einen Anlass gibt, und belege jede
mit einem wörtlichen Zitat. Die meisten Züge verändern nichts oder nur wenig -
eine leere Liste ist ein gutes Ergebnis.

Bewerte in kleinen Schritten: 1 für eine Kleinigkeit, 3 für etwas Merkliches,
5 für einen deutlichen Bruch oder eine deutliche Annäherung. Größere Werte
werden ohnehin gekappt.

Die Achsen: trust (Vertrauen), warmth (Zuneigung), attraction (Anziehung),
respect (Achtung), familiarity (Vertrautheit), tension (ungeklärte Spannung),
resentment (Groll), obligation (Schuld), fear (Furcht).`

var beziehungSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"deltas": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"figur":       map[string]any{"type": "string", "description": "wer empfindet"},
					"achse":       map[string]any{"type": "string"},
					"delta":       map[string]any{"type": "integer", "description": "-5 bis 5"},
					"begruendung": map[string]any{"type": "string"},
					"zitat":       map[string]any{"type": "string"},
				},
				"required":             []string{"figur", "achse", "delta", "begruendung", "zitat"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"deltas"},
	"additionalProperties": false,
}

// beziehungFortschreiben lässt den Analysten Veränderungen vorschlagen und
// trägt sie gedeckelt ein. Das Modell schlägt vor, die Engine entscheidet.
func (s *Server) beziehungFortschreiben(ctx context.Context, st *db.Story, knoten *db.Node,
	npcs []db.Character, set Settings) []db.Delta {

	if len(npcs) == 0 || strings.TrimSpace(knoten.Content) == "" {
		return nil
	}
	c, err := s.client()
	if err != nil {
		return nil
	}

	namen := make([]string, 0, len(npcs))
	fluechtig := map[string]float64{}
	for _, n := range npcs {
		namen = append(namen, n.Name)
		for a, v := range n.Sheet.Volatility {
			fluechtig[n.Name+"/"+a] = v
		}
	}

	null := 0.0
	res, err := c.Stream(ctx, openrouter.ChatRequest{
		Model: set.AnalystModel,
		Messages: []openrouter.Message{
			{Role: "system", Content: beziehungAnweisung + "\n\nAnwesend sind: " + strings.Join(namen, ", ")},
			{Role: "user", Content: knoten.Content},
		},
		MaxTokens:      700,
		Temperature:    &null,
		Provider:       analystProvider(set),
		ResponseFormat: openrouter.JSONAntwort("deltas", beziehungSchema),
	}, nil)
	if err != nil {
		s.db.LogRun(st.ID, &knoten.ID, "beziehung", set.AnalystModel, "{}", "{}", 0, 0, err.Error())
		return nil
	}

	var geparst struct {
		Deltas []struct {
			Figur       string `json:"figur"`
			Achse       string `json:"achse"`
			Delta       int    `json:"delta"`
			Begruendung string `json:"begruendung"`
			Zitat       string `json:"zitat"`
		} `json:"deltas"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Content)), &geparst); err != nil {
		s.db.LogRun(st.ID, &knoten.ID, "beziehung", res.Model, "{}", res.Content,
			res.Usage.Cost, res.Latency.Milliseconds(), "Antwort war kein gültiges JSON")
		return nil
	}

	bekannt := map[string]bool{}
	for _, n := range npcs {
		bekannt[strings.ToLower(n.Name)] = true
	}
	gueltigeAchse := map[string]bool{}
	for _, a := range db.Achsen {
		gueltigeAchse[a] = true
	}

	eingetragen := []db.Delta{}
	for _, d := range geparst.Deltas {
		if !bekannt[strings.ToLower(d.Figur)] || !gueltigeAchse[d.Achse] || d.Delta == 0 {
			continue
		}
		// Die Deckelung ist der eigentliche Schutz: Sie hält auch einen
		// überschießenden Analysten im Rahmen. Volatility pro Figur macht
		// manche Achsen träger als andere.
		grenze := float64(deckel)
		if f, gesetzt := fluechtig[d.Figur+"/"+d.Achse]; gesetzt && f > 0 {
			grenze *= f
		}
		wert := d.Delta
		if float64(wert) > grenze {
			wert = int(grenze)
		}
		if float64(wert) < -grenze {
			wert = -int(grenze)
		}
		if wert == 0 {
			continue
		}
		dl := db.Delta{StoryID: st.ID, NodeID: knoten.ID, Figur: d.Figur, Achse: d.Achse,
			Delta: wert, Begruendung: d.Begruendung, Zitat: d.Zitat}
		if err := s.db.AddDelta(dl); err == nil {
			eingetragen = append(eingetragen, dl)
		}
	}
	s.db.LogRun(st.ID, &knoten.ID, "beziehung", res.Model, "{}", res.Content,
		res.Usage.Cost, res.Latency.Milliseconds(), "")
	return eingetragen
}

// handleBeziehungen zeigt, wo die Verhältnisse gerade stehen und was sie
// zuletzt bewegt hat. Ohne diese Sicht lässt sich nicht beurteilen, ob die
// Fortschreibung vernünftig arbeitet oder davonläuft.
func (s *Server) handleBeziehungen(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	st, err := s.db.Story(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Geschichte nicht gefunden")
		return
	}
	var pfad []db.Node
	if st.HeadNodeID != nil {
		pfad, _ = s.db.Path(*st.HeadNodeID)
	}
	aufPfad := map[int64]bool{}
	for _, n := range pfad {
		aufPfad[n.ID] = true
	}
	_, npcs, _ := s.db.Anwesende(id)

	alle, _ := s.db.Deltas(id)
	// Nur was auf dieser Zeitlinie liegt, jüngste zuerst.
	verlauf := []db.Delta{}
	for i := len(alle) - 1; i >= 0 && len(verlauf) < 40; i-- {
		if aufPfad[alle[i].NodeID] {
			verlauf = append(verlauf, alle[i])
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"staende": s.staende(st, pfad, npcs),
		"verlauf": verlauf,
		"text":    s.rendereBeziehungen(st, pfad, npcs),
	})
}

func (s *Server) handleDeleteDelta(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	if err := s.db.DeleteDelta(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Eintrag nicht löschbar")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
