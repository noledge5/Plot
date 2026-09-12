package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/noledge5/plot/internal/db"
)

func (s *Server) handleBibliothek(w http.ResponseWriter, r *http.Request) {
	vs, err := s.db.Bibliothek()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Bibliothek nicht lesbar")
		return
	}
	writeJSON(w, http.StatusOK, vs)
}

// handleFigurMerken legt eine Figur aus einer Geschichte in der Bibliothek ab.
// Steht dort schon eine Figur dieses Namens, wird sie überschrieben - die
// Bibliothek soll je Person eine Fassung führen, nicht mehrere, die
// auseinanderlaufen.
func (s *Server) handleFigurMerken(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	c, err := s.db.Character(id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Figur nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Figur nicht lesbar")
		return
	}
	v, err := s.db.MerkeFigur(c.Name, c.Rolle, c.Sheet)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Figur konnte nicht gemerkt werden")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleFigurVergessen(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	if err := s.db.VergissFigur(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Figur konnte nicht entfernt werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleFigurUebernehmen legt eine Figur aus der Bibliothek in einer
// Geschichte an. Das Blatt wird kopiert, nicht verknüpft: Was die Figur in
// dieser Geschichte erlebt, darf nicht in eine andere durchschlagen.
func (s *Server) handleFigurUebernehmen(w http.ResponseWriter, r *http.Request) {
	storyID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültige Kennung")
		return
	}
	var in struct {
		VorlageID int64  `json:"vorlageId"`
		Name      string `json:"name"` // optional: unter anderem Namen übernehmen
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	v, err := s.db.FigurVorlage(in.VorlageID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Vorlage nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vorlage nicht lesbar")
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = v.Name
	}

	persona, npcs, err := s.db.Anwesende(storyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Figuren nicht lesbar")
		return
	}
	// Es gibt genau eine Spielerfigur je Geschichte.
	if v.Rolle == "persona" && persona != nil {
		writeError(w, http.StatusConflict, "es gibt bereits eine Spielerfigur")
		return
	}
	// Zweimal dieselbe Figur in einer Szene macht den Prompt widersprüchlich.
	for _, c := range append(npcs, personaAlsListe(persona)...) {
		if strings.EqualFold(c.Name, name) {
			writeError(w, http.StatusConflict, name+" steht in dieser Geschichte schon")
			return
		}
	}

	c, err := s.db.CreateCharacter(storyID, v.Rolle, name, v.Sheet)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Figur konnte nicht angelegt werden")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func personaAlsListe(p *db.Character) []db.Character {
	if p == nil {
		return nil
	}
	return []db.Character{*p}
}
