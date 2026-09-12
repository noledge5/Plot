package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/noledge5/plot/internal/db"
	"github.com/noledge5/plot/internal/openrouter"
)

// KindZeitsprung kennzeichnet einen Knoten, der eine Zeitspanne überbrückt.
const KindZeitsprung = "timeskip"

const zeitsprungAnweisung = `Zwischen zwei Szenen einer Rollenspiel-Erzählung vergeht Zeit.

Beschreibe in höchstens fünf Sätzen, was in dieser Zeit geschehen ist. Halte dich
an das, was die Figuren wollen: Wer ein Ziel verfolgt, kommt ihm näher oder
scheitert daran. Etwas verändert sich, aber nicht alles.

Die Figur des Spielers kommt darin nicht vor - er war nicht dabei. Erzähle nur,
was sich bei den anderen getan hat, und lass mindestens eine Sache offen.

Vergangenheitsform, dritte Person, keine Deutung.`

// handleZeitsprung überbrückt eine Zeitspanne. Der Text wird als eigener
// Knoten eingefügt und ist danach wie jeder andere änderbar - er ist ein
// Vorschlag der Engine, kein Gesetz.
func (s *Server) handleZeitsprung(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	var in struct {
		Spanne string `json:"spanne"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	if strings.TrimSpace(in.Spanne) == "" {
		in.Spanne = "einige Tage"
	}

	st, err := s.db.Story(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Geschichte nicht gefunden")
		return
	}
	if st.HeadNodeID == nil {
		writeError(w, http.StatusBadRequest, "es gibt noch nichts zu überspringen")
		return
	}
	c, err := s.client()
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, err.Error())
		return
	}

	set := s.settings()
	pfad, _ := s.db.Path(*st.HeadNodeID)
	_, npcs, _ := s.db.Anwesende(id)
	storySet := storySettings(st)

	var lage strings.Builder
	fmt.Fprintf(&lage, "Es vergehen: %s\n\n", strings.TrimSpace(in.Spanne))
	if chronik := s.rendereChronik(id, pfad); chronik != "" {
		lage.WriteString("Bisher:\n" + chronik + "\n\n")
	}
	if antriebe := rendereAntriebe(npcs, storySet.DruckSchwelle); antriebe != "" {
		lage.WriteString("Was die Figuren wollen:\n" + antriebe + "\n\n")
	}
	if len(pfad) > 0 {
		lage.WriteString("Zuletzt geschah:\n" + pfad[len(pfad)-1].Content)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	temp := 0.7
	res, err := c.Stream(ctx, openrouter.ChatRequest{
		Model: set.NarratorModel,
		Messages: []openrouter.Message{
			{Role: "system", Content: zeitsprungAnweisung},
			{Role: "user", Content: lage.String()},
		},
		MaxTokens:   500,
		Temperature: &temp,
		Provider:    narratorProvider(set),
	}, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	text := strings.TrimSpace(res.Content)
	if text == "" {
		writeError(w, http.StatusBadGateway, "der Erzähler hat nichts geliefert")
		return
	}
	verbrauch, _ := json.Marshal(res.Usage)
	knoten, err := s.db.AddNode(id, st.HeadNodeID, KindZeitsprung, "assistant",
		"("+strings.TrimSpace(in.Spanne)+" später)\n\n"+text, res.Model, string(verbrauch))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Zeitsprung nicht speicherbar")
		return
	}
	s.db.LogRun(id, &knoten.ID, "narrate", res.Model, "{}", text,
		res.Usage.Cost, res.Latency.Milliseconds(), "")

	writeJSON(w, http.StatusOK, knoten)
}

// handleWiedereinstieg beantwortet die Frage, mit der man nach ein paar Tagen
// vor der eigenen Geschichte steht: wo stehe ich, wer ist da, was ist offen.
// Ohne Modellaufruf - alles steht schon in der Datenbank.
func (s *Server) handleWiedereinstieg(w http.ResponseWriter, r *http.Request) {
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
	_, npcs, _ := s.db.Anwesende(id)

	// Der letzte Erzähltext ist der beste Anker: dort hört die Geschichte auf.
	letzter := ""
	for i := len(pfad) - 1; i >= 0; i-- {
		if pfad[i].Role == "assistant" {
			letzter = pfad[i].Content
			break
		}
	}
	namen := make([]string, 0, len(npcs))
	for _, c := range npcs {
		namen = append(namen, c.Name)
	}
	// Angeheftete Fakten sind das, was der Spieler selbst für wichtig hielt.
	offen := []db.Fact{}
	if alle, err := s.db.Facts(id, "canon"); err == nil {
		for _, f := range alle {
			if f.Angeheftet {
				offen = append(offen, f)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chronik":   s.rendereChronik(id, pfad),
		"zuletzt":   letzter,
		"anwesend":  namen,
		"wichtiges": offen,
		"zuege":     len(pfad),
	})
}
