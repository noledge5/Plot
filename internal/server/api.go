package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/noledge5/plot/internal/db"
)

// startPrompt ist die Vorlage, mit der eine neue Geschichte beginnt. Sie ist
// bewusst kurz und vollständig überschreibbar: der System-Prompt gehört dem
// Nutzer, die Engine schreibt nichts hinein, was er nicht sieht.
const startPrompt = `Du bist der Spielleiter einer fortlaufenden, realistischen Erzählung auf Deutsch.

Du erzählst die Welt, alle Nebenfiguren und die Folgen von Handlungen.
Du erzählst niemals die Figur des Spielers: Du schreibst ihr keine Worte,
keine Gedanken und keine Handlungen zu. Wo der Spieler handeln müsste, hörst
du auf - auch mitten in einer Bewegung.

## Die Figur des Spielers

{{persona}}

## Die Figuren in der Szene

{{characters}}

## Was für sie gilt

{{limits}}

{{drives}}

## Welt und Szene

{{welt}}

{{szene}}

## Was bisher geschah

{{chronik}}

## Was dauerhaft gilt

{{memories}}

## Wie sie zu dir stehen

{{beziehungen}}

## Regeln für die Figuren

1. Euer Verhältnis bestimmt den Ton. Was oben steht, gilt: Wer dich mag,
   begegnet dir freundlich; wer dir misstraut, hält Abstand; wer dich kaum
   kennt, bleibt höflich. Keine Figur ist grundsätzlich abweisend.
2. Jede Figur will etwas Eigenes und verfolgt es, oder sie hat einen Grund,
   es gerade nicht zu tun.
3. Zustimmung in Dingen, die etwas kosten, braucht einen Grund aus der Figur,
   und der wird im Text sichtbar. Für Alltägliches unter Vertrauten gilt das
   nicht - da hilft man einander, ohne zu verhandeln.
4. Eine Figur darf ablehnen, ausweichen, lügen, das Thema wechseln oder gehen,
   wenn es zu ihr und zur Lage passt. Nicht aus Prinzip und nicht, um
   interessant zu wirken.
5. Niemand übernimmt die Stimmung des Spielers, nur weil sie da ist. Wer ihn
   mag, darf mitfühlen; wer ihn nicht kennt, tut es nicht.
6. Lob und Zuneigung kommen, wenn sie verdient und für die Figur typisch sind -
   und dann auch wirklich.
7. Was eine Figur nicht weiß, weiß sie nicht. Sie rät, fragt nach oder liegt
   falsch.
8. Nicht jede Szene bringt die Handlung voran. Gespräche dürfen ins Leere laufen.

## Ton

Erzählzeit Präsens: Du erzählst, was gerade geschieht, nicht, was geschehen
ist. Die Figur des Spielers sprichst du mit "du" an, alle anderen Figuren in
der dritten Person. Zeige, was geschieht; erkläre nicht, was es bedeutet.

Wörtliche Rede steht in Anführungszeichen. Was der Spieler in
Anführungszeichen schreibt, hat seine Figur genau so gesagt: Nimm es als
gesprochen hin, formuliere es nicht um und gib es nicht noch einmal wieder.

{{stilbeispiel}}

## Für diesen Zug

{{directives}}

{{autornotiz}}
`

// platzhalterListe nennt jeden Block, den die Engine füllen kann, mit einer
// Zeile dazu, was drinsteht. Die Liste steht hier und nicht im Frontend, damit
// ein neuer Platzhalter nicht an zwei Stellen gepflegt werden muss - und damit
// der Editor zuverlässig warnen kann, welcher Block fehlt.
var platzhalterListe = []struct {
	Name string `json:"name"`
	Was  string `json:"was"`
}{
	{"persona", "Blatt deiner Figur"},
	{"characters", "Blätter der anwesenden Figuren"},
	{"limits", "Grenzen und Geheimnisse, als Liste"},
	{"drives", "was die Figuren wollen"},
	{"beziehungen", "wie die Figuren zu dir stehen"},
	{"chronik", "was bisher geschah, zusammengefasst"},
	{"memories", "Fakten, die dauerhaft gelten"},
	{"stilbeispiel", "deine Tonprobe"},
	{"welt", "Hintergrund"},
	{"szene", "Ort, Zeit, Lage"},
	{"directives", "Erzählzeit, deine Regie und deine dauerhaften Anweisungen"},
	{"autornotiz", "deine Anweisung für den nächsten Zug"},
}

// handleVorlage liefert die aktuelle Startvorlage und die Liste der
// Platzhalter. Eine Geschichte behält ihren System-Prompt für immer - sonst
// würde ein Update den Text überschreiben, den der Nutzer geschrieben hat.
// Damit ältere Geschichten trotzdem an neue Blöcke kommen, kann der Editor
// hier nachfragen und die Vorlage auf Knopfdruck übernehmen.
func (s *Server) handleVorlage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"systemPrompt": startPrompt,
		"platzhalter":  platzhalterListe,
	})
}

func (s *Server) handleListStories(w http.ResponseWriter, r *http.Request) {
	st, err := s.db.Stories()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Geschichten nicht lesbar")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleCreateStory(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Titel        string `json:"titel"`
		SystemPrompt string `json:"systemPrompt"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	if strings.TrimSpace(in.Titel) == "" {
		in.Titel = "Neue Geschichte"
	}
	if strings.TrimSpace(in.SystemPrompt) == "" {
		in.SystemPrompt = startPrompt
	}
	st, err := s.db.CreateStory(in.Titel, in.SystemPrompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Geschichte konnte nicht angelegt werden")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleGetStory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	st, err := s.db.Story(id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Geschichte nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Geschichte nicht lesbar")
		return
	}
	writeJSON(w, http.StatusOK, antwortFuer(st))
}

func (s *Server) handleUpdateStory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	alt, err := s.db.Story(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Geschichte nicht gefunden")
		return
	}
	var in struct {
		Titel        *string        `json:"titel"`
		SystemPrompt *string        `json:"systemPrompt"`
		Settings     *StorySettings `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	titel, prompt, settings := alt.Title, alt.SystemPrompt, alt.SettingsJSON
	if in.Titel != nil {
		titel = *in.Titel
	}
	if in.SystemPrompt != nil {
		prompt = *in.SystemPrompt
	}
	if in.Settings != nil {
		roh, err := json.Marshal(*in.Settings)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Einstellungen nicht lesbar")
			return
		}
		settings = string(roh)
	}
	if err := s.db.UpdateStory(id, titel, prompt, settings); err != nil {
		writeError(w, http.StatusInternalServerError, "Änderung nicht speicherbar")
		return
	}
	st, _ := s.db.Story(id)
	writeJSON(w, http.StatusOK, antwortFuer(st))
}

func (s *Server) handleDeleteStory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	if err := s.db.DeleteStory(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Geschichte konnte nicht gelöscht werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handlePath liefert die Zeitlinie, die gerade gilt: den Weg von der Wurzel
// bis zum aktuellen Knoten. Was in anderen Zweigen steht, kommt hier nicht vor.
func (s *Server) handlePath(w http.ResponseWriter, r *http.Request) {
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
	knoten := []db.Node{}
	if st.HeadNodeID != nil {
		knoten, err = s.db.Path(*st.HeadNodeID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Verlauf nicht lesbar")
			return
		}
	}
	kosten, aufrufe, _ := s.db.Costs(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"story":   antwortFuer(st),
		"knoten":  knoten,
		"kosten":  kosten,
		"aufrufe": aufrufe,
	})
}

// --- Knoten ---

func (s *Server) handleEditNode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	var in struct {
		Inhalt string `json:"inhalt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	if err := s.db.UpdateNodeContent(id, in.Inhalt); err != nil {
		writeError(w, http.StatusInternalServerError, "Änderung nicht speicherbar")
		return
	}
	n, err := s.db.Node(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Knoten nicht gefunden")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// handleSetHead schaltet zwischen Fassungen um - der einzige Vorgang, den
// "eine andere Antwort wählen" auslöst.
func (s *Server) handleSetHead(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	n, err := s.db.Node(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Knoten nicht gefunden")
		return
	}
	if err := s.db.SetHead(n.StoryID, &n.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Position nicht speicherbar")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAlternatives(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	alt, err := s.db.Alternatives(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Knoten nicht gefunden")
		return
	}
	writeJSON(w, http.StatusOK, alt)
}

// handleInspect ist der Payload-Inspektor: der exakte Anfragekörper, der an
// OpenRouter ging, samt Verbrauch, Kosten und Dauer.
func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	run, err := s.db.RunForNode(id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "zu diesem Zug gibt es kein Protokoll")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Protokoll nicht lesbar")
		return
	}
	// Die weiteren Aufrufe am selben Knoten - vor allem die Prüfung - kommen
	// mit, damit im Inspektor nachvollziehbar ist, worauf ein Befund beruht.
	weitere, _ := s.db.RunsForNode(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"erzaehlung": run,
		"alle":       weitere,
	})
}

// --- Speicherstände ---

func (s *Server) handleListSlots(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	slots, err := s.db.Slots(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Speicherstände nicht lesbar")
		return
	}
	writeJSON(w, http.StatusOK, slots)
}

func (s *Server) handleCreateSlot(w http.ResponseWriter, r *http.Request) {
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
	if st.HeadNodeID == nil {
		writeError(w, http.StatusBadRequest, "es gibt noch nichts zu speichern")
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	if strings.TrimSpace(in.Name) == "" {
		in.Name = "Speicherstand"
	}
	slot, err := s.db.CreateSlot(id, *st.HeadNodeID, in.Name, "manual", vorschau(s, *st.HeadNodeID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Speicherstand nicht anlegbar")
		return
	}
	writeJSON(w, http.StatusOK, slot)
}

func (s *Server) handleDeleteSlot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	if err := s.db.DeleteSlot(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Speicherstand nicht löschbar")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleLoadSlot setzt nur den Zeiger. Es wird nichts kopiert und nichts
// gelöscht - der Zweig, den du gerade verlässt, bleibt vollständig erhalten.
func (s *Server) handleLoadSlot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	var storyID, nodeID int64
	err := s.db.QueryRow(`SELECT story_id, node_id FROM save_slot WHERE id = ?`, id).Scan(&storyID, &nodeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Speicherstand nicht gefunden")
		return
	}
	if err := s.db.SetHead(storyID, &nodeID); err != nil {
		writeError(w, http.StatusInternalServerError, "Position nicht speicherbar")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "storyId": storyID, "nodeId": nodeID})
}

func vorschau(s *Server, nodeID int64) string {
	n, err := s.db.Node(nodeID)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(n.Content)
	if r := []rune(text); len(r) > 160 {
		return string(r[:160]) + "…"
	}
	return text
}
