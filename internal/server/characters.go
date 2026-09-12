package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/noledge5/plot/internal/db"
)

// storyAntwort reicht die Einstellungen als Objekt heraus statt als rohen
// JSON-Text - die Oberfläche soll nicht selbst parsen müssen.
type storyAntwort struct {
	*db.Story
	Settings StorySettings `json:"settings"`
}

func antwortFuer(st *db.Story) storyAntwort {
	return storyAntwort{Story: st, Settings: storySettings(st)}
}

func (s *Server) handleListCharacters(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	cs, err := s.db.Characters(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Figuren nicht lesbar")
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

func (s *Server) handleCreateCharacter(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	var in struct {
		Name  string   `json:"name"`
		Rolle string   `json:"rolle"`
		Sheet db.Sheet `json:"sheet"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "die Figur braucht einen Namen")
		return
	}
	if in.Rolle != "persona" {
		in.Rolle = "npc"
	}
	// Es gibt genau eine Spielerfigur je Geschichte; eine zweite würde den
	// Prompt widersprüchlich machen.
	if in.Rolle == "persona" {
		if vorhanden, _, err := s.db.Anwesende(id); err == nil && vorhanden != nil {
			writeError(w, http.StatusConflict, "es gibt bereits eine Spielerfigur")
			return
		}
	}
	c, err := s.db.CreateCharacter(id, in.Rolle, strings.TrimSpace(in.Name), in.Sheet)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Figur konnte nicht angelegt werden")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleUpdateCharacter(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	alt, err := s.db.Character(id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Figur nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Figur nicht lesbar")
		return
	}
	in := struct {
		Name       string   `json:"name"`
		Sheet      db.Sheet `json:"sheet"`
		Aktiv      bool     `json:"aktiv"`
		Sortierung int      `json:"sortierung"`
	}{Name: alt.Name, Sheet: alt.Sheet, Aktiv: alt.Aktiv, Sortierung: alt.Sortierung}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "die Figur braucht einen Namen")
		return
	}
	if err := s.db.UpdateCharacter(id, strings.TrimSpace(in.Name), in.Sheet, in.Aktiv, in.Sortierung); err != nil {
		writeError(w, http.StatusInternalServerError, "Änderung nicht speicherbar")
		return
	}
	c, _ := s.db.Character(id)
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteCharacter(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	if err := s.db.DeleteCharacter(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Figur konnte nicht gelöscht werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handlePreview zeigt, was mit den aktuellen Figuren und Einstellungen als
// System-Prompt herauskäme - ohne dafür einen Zug verbrauchen zu müssen.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
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
	// Erlaubt die Vorschau eines noch nicht gespeicherten Entwurfs.
	var in struct {
		SystemPrompt *string `json:"systemPrompt"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	vorlage := st.SystemPrompt
	if in.SystemPrompt != nil {
		vorlage = *in.SystemPrompt
	}

	persona, npcs, err := s.db.Anwesende(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Figuren nicht lesbar")
		return
	}
	var pfad []db.Node
	if st.HeadNodeID != nil {
		pfad, _ = s.db.Path(*st.HeadNodeID)
	}
	text, bloecke := baueSystemPrompt(vorlage, promptWerte(st, storySettings(st), persona, npcs, nil,
		s.rendereChronik(st.ID, pfad), s.abrufFakten(st, pfad, npcs, 12),
		s.rendereBeziehungen(st, pfad, npcs)))
	writeJSON(w, http.StatusOK, map[string]any{
		"text":    text,
		"bloecke": bloecke,
		"tokens":  schaetzeTokens(text),
	})
}
