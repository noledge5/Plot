package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/noledge5/plot/internal/db"
)

func (s *Server) handleListFacts(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	fakten, err := s.db.Facts(id, r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Fakten nicht lesbar")
		return
	}
	writeJSON(w, http.StatusOK, fakten)
}

func (s *Server) handleCreateFact(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	var in db.Fact
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	if strings.TrimSpace(in.Text) == "" {
		writeError(w, http.StatusBadRequest, "der Fakt braucht einen Text")
		return
	}
	in.StoryID = id
	// Von Hand angelegte Fakten gelten sofort und ohne Bindung an einen Zug:
	// sie sind eine Setzung des Spielers, keine Ableitung aus dem Text.
	in.Status = "canon"
	in.GiltAb = nil
	f, err := s.db.CreateFact(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Fakt konnte nicht angelegt werden")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleUpdateFact(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	alt, err := s.db.Fact(id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Fakt nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Fakt nicht lesbar")
		return
	}
	in := *alt
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	in.ID, in.StoryID = alt.ID, alt.StoryID
	if strings.TrimSpace(in.Text) == "" {
		writeError(w, http.StatusBadRequest, "der Fakt braucht einen Text")
		return
	}
	if err := s.db.UpdateFact(in); err != nil {
		writeError(w, http.StatusInternalServerError, "Änderung nicht speicherbar")
		return
	}
	f, _ := s.db.Fact(id)
	writeJSON(w, http.StatusOK, f)
}

// handleRetireFact setzt einen Fakt außer Kraft, statt ihn zu löschen - so
// bleibt darstellbar, dass er einmal galt.
func (s *Server) handleRetireFact(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	f, err := s.db.Fact(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Fakt nicht gefunden")
		return
	}
	st, err := s.db.Story(f.StoryID)
	if err != nil || st.HeadNodeID == nil {
		writeError(w, http.StatusBadRequest, "die Geschichte hat noch keinen Verlauf")
		return
	}
	if err := s.db.Zurueckziehen(id, *st.HeadNodeID); err != nil {
		writeError(w, http.StatusInternalServerError, "Fakt konnte nicht stillgelegt werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteFact(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	if err := s.db.DeleteFact(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Fakt konnte nicht gelöscht werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleChronik liefert die Zusammenfassungen dieser Zeitlinie, dazu die
// Fakten, die gerade in den Prompt gingen - so ist sichtbar, woran sich die
// Erzählung gerade erinnert.
func (s *Server) handleChronik(w http.ResponseWriter, r *http.Request) {
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

	alle, _ := s.db.Summaries(id)
	aufPfad := map[int64]bool{}
	for _, n := range pfad {
		aufPfad[n.ID] = true
	}
	gueltigeZusammen := []db.Summary{}
	for _, z := range alle {
		if aufPfad[z.BisNode] {
			gueltigeZusammen = append(gueltigeZusammen, z)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chronik":   gueltigeZusammen,
		"imPrompt":  s.abrufFakten(st, pfad, npcs, 12),
		"bisKnoten": len(pfad),
	})
}

func (s *Server) handleDeleteSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	if err := s.db.DeleteSummary(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Eintrag konnte nicht gelöscht werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
