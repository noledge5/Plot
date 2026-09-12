package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/noledge5/plot/internal/db"
	"github.com/noledge5/plot/internal/openrouter"
)

// verschaerfung ist die Direktive hinter "nochmal, härter". Sie beschreibt,
// was zu tun ist, statt zu verbieten - positive Anweisungen werden von kleinen
// Modellen zuverlässiger befolgt als Verbote.
const verschaerfung = `Für diesen Zug gilt zusätzlich: Die Figur gibt nicht nach. ` +
	`Sie hat einen eigenen Grund, warum nicht, und sie benennt ihn. ` +
	`Sie spiegelt die Stimmung des Spielers nicht, lobt ihn nicht und ` +
	`beantwortet keine heikle Frage bereitwillig. Wenn sie etwas will, ` +
	`bringt sie es jetzt zur Sprache.`

// autosaveAlle legt in diesem Abstand einen automatischen Speicherstand an,
// autosaveBehalten begrenzt, wie viele davon aufgehoben werden.
const (
	autosaveAlle     = 5
	autosaveBehalten = 5
)

// sse bündelt das Schreiben von Server-Sent-Events. Beim Modellvergleich
// schreiben zwei Goroutinen in dieselbe Verbindung, deshalb der Mutex.
type sse struct {
	mu sync.Mutex
	w  http.ResponseWriter
	rc *http.ResponseController
}

func neuesSSE(w http.ResponseWriter) *sse {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Verhindert, dass ein zwischengeschalteter Reverse Proxy puffert und der
	// Text erst am Ende auf einen Schlag ankommt.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	s := &sse{w: w, rc: http.NewResponseController(w)}
	s.rc.Flush()
	return s
}

func (s *sse) sende(event string, daten any) {
	b, err := json.Marshal(daten)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, b)
	s.rc.Flush()
}

// erzaehlung führt einen einzelnen Erzählaufruf aus: streamen, Knoten anlegen,
// Verbrauch und Protokoll sichern. Der Knoten entsteht auch dann, wenn der
// Nutzer abbricht - der Text bis dahin bleibt erhalten.
func (s *Server) erzaehlung(ctx context.Context, st *db.Story, elternID *int64, modell string,
	set Settings, direktiven []string, onDelta func(string)) (*db.Node, *openrouter.Result, error) {

	c, err := s.client()
	if err != nil {
		return nil, nil, err
	}

	var pfad []db.Node
	if elternID != nil {
		pfad, err = s.db.Path(*elternID)
		if err != nil {
			return nil, nil, fmt.Errorf("Verlauf lesen: %w", err)
		}
	}

	// Figuren, Grenzen und Antriebe werden in die Vorlage des Nutzers
	// eingesetzt - die Engine hängt nichts an, was nicht als Platzhalter
	// dort steht.
	storySet := storySettings(st)
	persona, npcs, err := s.db.Anwesende(st.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("Figuren lesen: %w", err)
	}
	// Regieanweisungen aus dem Verlauf kommen zu den Direktiven der Engine
	// dazu und landen gemeinsam in {{directives}}. Die Zeitform steht vorn:
	// Sie gilt für jeden Zug, alles andere nur für diesen.
	direktiven = append(offeneRegie(pfad), direktiven...)
	if satz := zeitDirektive[storySet.Erzaehlzeit]; satz != "" {
		direktiven = append([]string{satz}, direktiven...)
	}

	// Chronik und Faktenblatt: was aus dem wörtlichen Verlauf gefallen ist,
	// und was dauerhaft gilt.
	chronik := s.rendereChronik(st.ID, pfad)
	fakten := s.abrufFakten(st, pfad, npcs, 12)
	// Der Beziehungsblock steht im Prompt vor den Regeln: Er setzt den Ton,
	// die Regeln wandeln ihn nur ab.
	beziehungen := s.rendereBeziehungen(st, pfad, npcs)

	system, bloecke := baueSystemPrompt(st.SystemPrompt,
		promptWerte(st, storySet, persona, npcs, direktiven, chronik, fakten, beziehungen))
	msgs, abgeschnitten := baueNachrichten(system, pfad, set.MaxHistoryTurns)

	temp := set.Temperature
	anfrage := openrouter.ChatRequest{
		Model:       modell,
		Messages:    msgs,
		MaxTokens:   set.MaxTokens,
		Temperature: &temp,
		Provider:    narratorProvider(set),
	}
	// Der exakte Anfragekörper wandert ins Protokoll - ohne ihn lässt sich
	// nie klären, ob eine schwache Antwort am Modell oder am Prompt lag.
	roh, _ := json.MarshalIndent(struct {
		openrouter.ChatRequest
		Abgeschnitten int     `json:"_abgeschnitteneZuege"`
		Bloecke       []Block `json:"_bloecke"`
	}{anfrage, abgeschnitten, bloecke}, "", "  ")

	res, streamErr := c.Stream(ctx, anfrage, onDelta)
	if res == nil {
		s.db.LogRun(st.ID, nil, "narrate", modell, string(roh), "{}", 0, 0, streamErr.Error())
		return nil, nil, streamErr
	}

	verbrauch, _ := json.Marshal(res.Usage)
	knoten, err := s.db.AddNode(st.ID, elternID, "turn", "assistant", res.Content, res.Model, string(verbrauch))
	if err != nil {
		return nil, res, fmt.Errorf("Antwort speichern: %w", err)
	}
	antwortMeta, _ := json.Marshal(map[string]any{
		"usage": res.Usage, "provider": res.Provider, "model": res.Model,
	})
	meldung := ""
	if streamErr != nil {
		meldung = streamErr.Error()
	}
	s.db.LogRun(st.ID, &knoten.ID, "narrate", modell, string(roh), string(antwortMeta),
		res.Usage.Cost, res.Latency.Milliseconds(), meldung)
	return knoten, res, nil
}

// handleTurn ist der normale Spielzug: Eingabe anhängen, Antwort streamen.
func (s *Server) handleTurn(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	var in struct {
		Text   string `json:"text"`
		Modell string `json:"modell"`
		// Modus "regie" heißt: Der Text ist eine Anweisung an den Erzähler,
		// keine Handlung der Spielerfigur. Modus "dialog" heißt: Der Text ist
		// wörtliche Rede und geht in Anführungszeichen raus.
		Modus string `json:"modus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	st, err := s.db.Story(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Geschichte nicht gefunden")
		return
	}
	// Vorbedingungen vor den SSE-Kopfzeilen prüfen, damit echte Fehler noch
	// als sauberer HTTP-Status ankommen.
	if _, err := s.client(); err != nil {
		writeError(w, http.StatusPreconditionRequired, err.Error())
		return
	}
	set := s.settings()
	modell := set.NarratorModel
	if strings.TrimSpace(in.Modell) != "" {
		modell = in.Modell
	}

	art := "turn"
	switch in.Modus {
	case KindRegie:
		art = KindRegie
	case KindDialog:
		art = KindDialog
	}

	eltern := st.HeadNodeID
	if strings.TrimSpace(in.Text) != "" {
		eingabe, err := s.db.AddNode(id, eltern, art, "user", strings.TrimSpace(in.Text), "", "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Eingabe konnte nicht gespeichert werden")
			return
		}
		eltern = &eingabe.ID
	} else if eltern == nil {
		writeError(w, http.StatusBadRequest, "die erste Eingabe darf nicht leer sein")
		return
	}

	strom := neuesSSE(w)
	if eltern != nil {
		strom.sende("eingabe", map[string]any{"nodeId": *eltern})
	}

	knoten, res, err := s.erzaehlung(r.Context(), st, eltern, modell, set, nil,
		func(t string) { strom.sende("delta", map[string]any{"text": t}) })
	if err != nil {
		strom.sende("fehler", map[string]any{"fehler": err.Error()})
		return
	}

	s.autosave(st.ID, knoten.ID)
	kosten, aufrufe, _ := s.db.Costs(st.ID)
	strom.sende("fertig", map[string]any{
		"nodeId": knoten.ID, "modell": res.Model, "anbieter": res.Provider,
		"usage": res.Usage, "kostenGesamt": kosten, "aufrufe": aufrufe,
	})

	// Erst der Text, dann die Auswertung: der Leser soll nicht auf den
	// Analysten warten. Die Verbindung steht noch, also kommt beides nach.
	s.pruefungNachreichen(strom, st, knoten, set)
	s.gedaechtnisNachreichen(strom, st, knoten, set)
	s.beziehungNachreichen(strom, st, knoten, set)
}

// pruefungNachreichen lässt den Detektor laufen und schickt das Ergebnis über
// dieselbe Verbindung nach.
func (s *Server) pruefungNachreichen(strom *sse, st *db.Story, knoten *db.Node, set Settings) {
	if !set.DetektorAn || strings.TrimSpace(set.AnalystModel) == "" {
		return
	}
	flags := s.detektorLauf(st, knoten, set)
	strom.sende("befunde", map[string]any{"nodeId": knoten.ID, "flags": flags})
}

// gedaechtnisNachreichen gewinnt Fakten und verlängert die Chronik, nachdem
// der Text beim Leser ist.
func (s *Server) gedaechtnisNachreichen(strom *sse, st *db.Story, knoten *db.Node, set Settings) {
	if !set.GedaechtnisAn {
		return
	}
	neu := s.gedaechtnisNachziehen(st, knoten, set)
	if neu > 0 {
		strom.sende("gedaechtnis", map[string]any{"nodeId": knoten.ID, "neueFakten": neu})
	}
}

// beziehungNachreichen schreibt die Verhältnisse fort und meldet, was sich
// bewegt hat.
func (s *Server) beziehungNachreichen(strom *sse, st *db.Story, knoten *db.Node, set Settings) {
	if !set.BeziehungenAn || strings.TrimSpace(set.AnalystModel) == "" {
		return
	}
	_, npcs, err := s.db.Anwesende(st.ID)
	if err != nil || len(npcs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	deltas := s.beziehungFortschreiben(ctx, st, knoten, npcs, set)
	if len(deltas) == 0 {
		return
	}
	pfad, _ := s.db.Path(knoten.ID)
	strom.sende("beziehung", map[string]any{
		"nodeId": knoten.ID, "deltas": deltas, "staende": s.staende(st, pfad, npcs),
	})
}

// handleRegenerate erzeugt eine weitere Fassung desselben Zuges. Die bisherige
// bleibt als Geschwisterknoten erhalten.
func (s *Server) handleRegenerate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	var in struct {
		NodeID int64  `json:"nodeId"`
		Modell string `json:"modell"`
		// Haerter setzt eine Direktive gegen Gefälligkeit in den Prompt -
		// der Knopf am Befund des Detektors.
		Haerter bool `json:"haerter"`
	}
	json.NewDecoder(r.Body).Decode(&in)

	st, err := s.db.Story(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Geschichte nicht gefunden")
		return
	}
	ziel := in.NodeID
	if ziel == 0 && st.HeadNodeID != nil {
		ziel = *st.HeadNodeID
	}
	alt, err := s.db.Node(ziel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "es gibt nichts zu wiederholen")
		return
	}
	if alt.Role != "assistant" {
		writeError(w, http.StatusBadRequest, "nur eine Antwort des Erzählers lässt sich wiederholen")
		return
	}
	if _, err := s.client(); err != nil {
		writeError(w, http.StatusPreconditionRequired, err.Error())
		return
	}

	set := s.settings()
	modell := set.NarratorModel
	if strings.TrimSpace(in.Modell) != "" {
		modell = in.Modell // erlaubt den Wechsel auf das Reservemodell
	}

	var direktiven []string
	if in.Haerter {
		direktiven = append(direktiven, verschaerfung)
	}

	strom := neuesSSE(w)
	knoten, res, err := s.erzaehlung(r.Context(), st, alt.ParentID, modell, set, direktiven,
		func(t string) { strom.sende("delta", map[string]any{"text": t}) })
	if err != nil {
		strom.sende("fehler", map[string]any{"fehler": err.Error()})
		return
	}
	kosten, aufrufe, _ := s.db.Costs(st.ID)
	strom.sende("fertig", map[string]any{
		"nodeId": knoten.ID, "ersetzt": alt.ID, "modell": res.Model, "anbieter": res.Provider,
		"usage": res.Usage, "kostenGesamt": kosten, "aufrufe": aufrufe,
	})
	s.pruefungNachreichen(strom, st, knoten, set)
	s.gedaechtnisNachreichen(strom, st, knoten, set)
	s.beziehungNachreichen(strom, st, knoten, set)
}

// handleCompare stellt zwei Modelle nebeneinander: derselbe Prompt, zwei
// Antworten, beide als Fassungen desselben Zuges. Für Deutsch ist das kein
// Komfort, sondern die einzige belastbare Art, ein Modell auszuwählen.
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	var in struct {
		Text    string `json:"text"`
		ModellA string `json:"modellA"`
		ModellB string `json:"modellB"`
		NodeID  int64  `json:"nodeId"` // gesetzt: vorhandenen Zug vergleichen
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	st, err := s.db.Story(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Geschichte nicht gefunden")
		return
	}
	if _, err := s.client(); err != nil {
		writeError(w, http.StatusPreconditionRequired, err.Error())
		return
	}
	set := s.settings()
	if strings.TrimSpace(in.ModellA) == "" {
		in.ModellA = set.NarratorModel
	}
	if strings.TrimSpace(in.ModellB) == "" {
		in.ModellB = set.ReserveModel
	}
	if in.ModellA == in.ModellB {
		writeError(w, http.StatusBadRequest, "für einen Vergleich braucht es zwei verschiedene Modelle")
		return
	}

	var eltern *int64
	switch {
	case in.NodeID != 0:
		alt, err := s.db.Node(in.NodeID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Zug nicht gefunden")
			return
		}
		eltern = alt.ParentID
	case strings.TrimSpace(in.Text) != "":
		eingabe, err := s.db.AddNode(id, st.HeadNodeID, "turn", "user", strings.TrimSpace(in.Text), "", "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Eingabe konnte nicht gespeichert werden")
			return
		}
		eltern = &eingabe.ID
	default:
		eltern = st.HeadNodeID
		if eltern == nil {
			writeError(w, http.StatusBadRequest, "es gibt noch nichts zu vergleichen")
			return
		}
	}

	strom := neuesSSE(w)
	if eltern != nil {
		strom.sende("eingabe", map[string]any{"nodeId": *eltern})
	}

	var wg sync.WaitGroup
	lauf := func(seite, modell string) {
		defer wg.Done()
		knoten, res, err := s.erzaehlung(r.Context(), st, eltern, modell, set, nil,
			func(t string) { strom.sende("delta", map[string]any{"seite": seite, "text": t}) })
		if err != nil {
			strom.sende("fehler", map[string]any{"seite": seite, "fehler": err.Error()})
			return
		}
		strom.sende("seiteFertig", map[string]any{
			"seite": seite, "nodeId": knoten.ID, "modell": res.Model,
			"anbieter": res.Provider, "usage": res.Usage,
		})
	}
	wg.Add(2)
	go lauf("a", in.ModellA)
	go lauf("b", in.ModellB)
	wg.Wait()

	// Nach einem Vergleich steht die Position noch auf einer der beiden
	// Fassungen. Welche gilt, entscheidet der Nutzer per Auswahl.
	kosten, aufrufe, _ := s.db.Costs(st.ID)
	strom.sende("fertig", map[string]any{"kostenGesamt": kosten, "aufrufe": aufrufe})
}

// autosave legt in festem Abstand einen automatischen Speicherstand an, damit
// ein versehentliches Weiterspielen nie eine gute Stelle verschluckt.
func (s *Server) autosave(storyID, nodeID int64) {
	var zuege int
	if err := s.db.QueryRow(`SELECT count(*) FROM node WHERE story_id = ? AND role = 'assistant'`,
		storyID).Scan(&zuege); err != nil {
		return
	}
	if zuege == 0 || zuege%autosaveAlle != 0 {
		return
	}
	if _, err := s.db.CreateSlot(storyID, nodeID, fmt.Sprintf("Automatisch (Zug %d)", zuege),
		"auto", vorschau(s, nodeID)); err != nil {
		return
	}
	s.db.PruneAutoSlots(storyID, autosaveBehalten)
}
