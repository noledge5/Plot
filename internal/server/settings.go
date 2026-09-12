package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/noledge5/plot/internal/openrouter"
)

// Settings sind die Betriebseinstellungen, die in der Oberfläche gesetzt
// werden. Die Vorgaben bilden die Entscheidungen aus docs/modelle.md ab:
// unzensierter Erzähler über eine Anbieter-Allowlist, Analyst mit striktem
// JSON und erzwungenem Zero Data Retention.
type Settings struct {
	NarratorModel     string   `json:"narratorModel"`
	ReserveModel      string   `json:"reserveModel"`
	AnalystModel      string   `json:"analystModel"`
	NarratorProviders []string `json:"narratorProviders"`
	NarratorZDR       bool     `json:"narratorZdr"`
	AnalystZDR        bool     `json:"analystZdr"`
	// MaxHistoryTurns hält den Kontext bewusst klein. Kleine Modelle verlieren
	// Anweisungen in langen Prompts - mehr Verlauf heißt schlechtere
	// Regelbefolgung, nicht bessere (siehe PLAN.md 5.1).
	MaxHistoryTurns int     `json:"maxHistoryTurns"`
	MaxTokens       int     `json:"maxTokens"`
	Temperature     float64 `json:"temperature"`
	// DetektorAn schaltet die Prüfung auf Gefälligkeit ein. Sie läuft nach
	// dem Streaming und kostet rund ein Hundertstel Cent je Zug.
	DetektorAn bool `json:"detektorAn"`
	// GedaechtnisAn schaltet Chronik und Faktenblatt ein: Was aus dem
	// wörtlichen Verlauf fällt, wird zusammengefasst statt vergessen.
	GedaechtnisAn bool `json:"gedaechtnisAn"`
	// FaktenAutomatisch übernimmt neue Fakten ohne Rückfrage. Aus heißt:
	// sie landen als Vorschlag und gelten erst nach deiner Freigabe.
	FaktenAutomatisch bool `json:"faktenAutomatisch"`
	// BeziehungenAn schreibt nach jedem Zug fort, wie die Figuren zum Spieler
	// stehen - gedeckelt, damit ein Satz kein Verhältnis umwirft.
	BeziehungenAn bool `json:"beziehungenAn"`
}

func defaultSettings() Settings {
	return Settings{
		NarratorModel:     "cognitivecomputations/dolphin-mistral-24b-venice-edition",
		ReserveModel:      "mistralai/mistral-small-2603",
		AnalystModel:      "mistralai/mistral-nemo",
		NarratorProviders: []string{"venice"},
		NarratorZDR:       false, // Venice führt keinen ZDR-Endpunkt (E16)
		AnalystZDR:        true,
		MaxHistoryTurns:   20,
		MaxTokens:         1200,
		Temperature:       0.9,
		DetektorAn:        true,
		GedaechtnisAn:     true,
		FaktenAutomatisch: true,
		BeziehungenAn:     true,
	}
}

func (s *Server) settings() Settings {
	set := defaultSettings()
	roh := s.db.Setting("app_settings", "")
	if roh != "" {
		// Unbekannte oder fehlende Felder behalten ihren Vorgabewert.
		_ = json.Unmarshal([]byte(roh), &set)
	}
	return set
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	set := s.settings()
	writeJSON(w, http.StatusOK, map[string]any{
		"settings":   set,
		"keyGesetzt": s.db.Setting("openrouter_key", "") != "",
	})
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Settings Settings `json:"settings"`
		// Key wird nur gesetzt, wenn ein nicht-leerer Wert kommt; die
		// Oberfläche bekommt ihn nie zurück und schickt ihn nur beim Ändern.
		Key *string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	set := in.Settings
	if set.MaxHistoryTurns < 2 {
		set.MaxHistoryTurns = 2
	}
	if set.MaxHistoryTurns > 200 {
		set.MaxHistoryTurns = 200
	}
	if set.MaxTokens < 64 {
		set.MaxTokens = 64
	}
	if set.Temperature < 0 || set.Temperature > 2 {
		set.Temperature = defaultSettings().Temperature
	}
	if strings.TrimSpace(set.NarratorModel) == "" {
		writeError(w, http.StatusBadRequest, "es muss ein Erzählmodell gesetzt sein")
		return
	}
	b, err := json.Marshal(set)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Einstellungen nicht speicherbar")
		return
	}
	if err := s.db.SetSetting("app_settings", string(b)); err != nil {
		writeError(w, http.StatusInternalServerError, "Einstellungen nicht speicherbar")
		return
	}
	if in.Key != nil && strings.TrimSpace(*in.Key) != "" {
		if err := s.db.SetSetting("openrouter_key", strings.TrimSpace(*in.Key)); err != nil {
			writeError(w, http.StatusInternalServerError, "Schlüssel nicht speicherbar")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	c, err := s.client()
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	ms, err := c.Models(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ms)
}

// narratorProvider baut die Routing-Regeln für die Erzählung: nur die
// erlaubten Anbieter, keine stillen Ausweichrouten.
func narratorProvider(set Settings) *openrouter.Provider {
	p := &openrouter.Provider{}
	leer := true
	if len(set.NarratorProviders) > 0 {
		p.Only = set.NarratorProviders
		nein := false
		p.AllowFallbacks = &nein
		leer = false
	}
	if set.NarratorZDR {
		ja := true
		p.ZDR = &ja
		p.DataCollection = "deny"
		leer = false
	}
	if leer {
		return nil
	}
	return p
}

// analystProvider erzwingt Zero Data Retention für die Auswertungsaufrufe.
func analystProvider(set Settings) *openrouter.Provider {
	if !set.AnalystZDR {
		return nil
	}
	ja := true
	return &openrouter.Provider{ZDR: &ja, DataCollection: "deny"}
}

// handleRoutingTest beantwortet die offene Frage aus M0: Kommt das gewählte
// Modell unter den eingestellten Bedingungen bei einem Anbieter an?
func (s *Server) handleRoutingTest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Rolle string `json:"rolle"` // narrator | reserve | analyst
	}
	json.NewDecoder(r.Body).Decode(&in)

	c, err := s.client()
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, err.Error())
		return
	}
	set := s.settings()

	var modell string
	var prov *openrouter.Provider
	switch in.Rolle {
	case "analyst":
		modell, prov = set.AnalystModel, analystProvider(set)
	case "reserve":
		modell, prov = set.ReserveModel, narratorProvider(set)
	default:
		in.Rolle = "narrator"
		modell, prov = set.NarratorModel, narratorProvider(set)
	}
	if strings.TrimSpace(modell) == "" {
		writeError(w, http.StatusBadRequest, "für diese Rolle ist kein Modell gesetzt")
		return
	}

	anbieter, err := c.CheckRouting(r.Context(), modell, prov)
	if err != nil {
		// Kein 5xx: eine abgelehnte Routing-Kombination ist ein Ergebnis,
		// keine Störung. Die Oberfläche zeigt den Wortlaut an.
		writeJSON(w, http.StatusOK, map[string]any{
			"rolle": in.Rolle, "modell": modell, "ok": false, "fehler": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rolle": in.Rolle, "modell": modell, "ok": true, "anbieter": anbieter,
	})
}
