// Package server stellt die HTTP-Schnittstelle und die Oberfläche bereit.
package server

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/noledge5/plot/internal/config"
	"github.com/noledge5/plot/internal/db"
	"github.com/noledge5/plot/internal/openrouter"
	"github.com/noledge5/plot/internal/webui"
)

type Server struct {
	db     *db.DB
	cfg    *config.Config
	log    *slog.Logger
	bremse *bremse
	mux    *http.ServeMux
}

func New(cfg *config.Config, database *db.DB, log *slog.Logger) *Server {
	s := &Server{db: database, cfg: cfg, log: log, bremse: &bremse{}, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) routes() {
	// Offen, weil ohne sie niemand hereinkommt.
	s.mux.HandleFunc("GET /api/state", s.handleState)
	s.mux.HandleFunc("POST /api/setup", s.handleSetup)
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/logout", s.handleLogout)

	geschuetzt := http.NewServeMux()
	geschuetzt.HandleFunc("GET /api/settings", s.handleGetSettings)
	geschuetzt.HandleFunc("PUT /api/settings", s.handlePutSettings)
	geschuetzt.HandleFunc("GET /api/models", s.handleModels)
	geschuetzt.HandleFunc("POST /api/routing-test", s.handleRoutingTest)

	geschuetzt.HandleFunc("GET /api/stories", s.handleListStories)
	geschuetzt.HandleFunc("POST /api/stories", s.handleCreateStory)
	geschuetzt.HandleFunc("GET /api/stories/{id}", s.handleGetStory)
	geschuetzt.HandleFunc("PUT /api/stories/{id}", s.handleUpdateStory)
	geschuetzt.HandleFunc("DELETE /api/stories/{id}", s.handleDeleteStory)
	geschuetzt.HandleFunc("GET /api/stories/{id}/path", s.handlePath)
	geschuetzt.HandleFunc("GET /api/stories/{id}/characters", s.handleListCharacters)
	geschuetzt.HandleFunc("POST /api/stories/{id}/characters", s.handleCreateCharacter)
	geschuetzt.HandleFunc("PUT /api/characters/{id}", s.handleUpdateCharacter)
	geschuetzt.HandleFunc("DELETE /api/characters/{id}", s.handleDeleteCharacter)
	geschuetzt.HandleFunc("POST /api/stories/{id}/preview", s.handlePreview)
	geschuetzt.HandleFunc("GET /api/stories/{id}/slots", s.handleListSlots)
	geschuetzt.HandleFunc("POST /api/stories/{id}/slots", s.handleCreateSlot)
	geschuetzt.HandleFunc("DELETE /api/slots/{id}", s.handleDeleteSlot)
	geschuetzt.HandleFunc("POST /api/slots/{id}/load", s.handleLoadSlot)

	geschuetzt.HandleFunc("PUT /api/nodes/{id}", s.handleEditNode)
	geschuetzt.HandleFunc("POST /api/nodes/{id}/head", s.handleSetHead)
	geschuetzt.HandleFunc("GET /api/nodes/{id}/alternatives", s.handleAlternatives)
	geschuetzt.HandleFunc("GET /api/nodes/{id}/inspect", s.handleInspect)

	geschuetzt.HandleFunc("POST /api/stories/{id}/turn", s.handleTurn)
	geschuetzt.HandleFunc("POST /api/stories/{id}/regenerate", s.handleRegenerate)
	geschuetzt.HandleFunc("POST /api/stories/{id}/compare", s.handleCompare)

	s.mux.Handle("/api/", s.requireAuth(geschuetzt))
	s.mux.Handle("/", s.static())
}

// ohneOberflaeche erklärt, was fehlt, statt eine leere Seite auszuliefern.
// Das passiert nur in einem Arbeitsbaum, in dem das Frontend nie gebaut wurde -
// ausgelieferte Binaries tragen es immer in sich.
const ohneOberflaeche = `<!doctype html><meta charset="utf-8"><title>Plot</title>
<p style="font:16px system-ui;margin:3rem auto;max-width:34rem">
Dieses Binary wurde ohne Oberfläche gebaut. Die Schnittstelle unter <code>/api/</code>
funktioniert. Für die Oberfläche im Ordner <code>web/</code> einmal
<code>npm install &amp;&amp; npm run build</code> ausführen und das Binary neu bauen.</p>`

// static liefert die eingebettete Oberfläche. Unbekannte Pfade bekommen die
// index.html, damit die Navigation im Browser funktioniert.
func (s *Server) static() http.Handler {
	if !webui.Vorhanden() {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(ohneOberflaeche))
		})
	}
	inhalt := webui.FS()
	datei := http.FileServer(http.FS(inhalt))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pfad := strings.TrimPrefix(r.URL.Path, "/")
		if pfad == "" {
			pfad = "index.html"
		}
		if _, err := fs.Stat(inhalt, pfad); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		datei.ServeHTTP(w, r)
	})
}

// --- Hilfsfunktionen ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Der Header ist raus, mehr als protokollieren geht hier nicht.
		slog.Debug("Antwort konnte nicht geschrieben werden", "fehler", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"fehler": msg})
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// client baut einen OpenRouter-Zugang aus dem hinterlegten Schlüssel. Die
// Basisadresse lässt sich überschreiben - für Tests und für den Fall, dass
// die Anfragen über einen eigenen Proxy laufen sollen.
func (s *Server) client() (*openrouter.Client, error) {
	key := s.db.Setting("openrouter_key", "")
	if strings.TrimSpace(key) == "" {
		return nil, errKeinKey
	}
	c := openrouter.New(key)
	if basis := s.db.Setting("openrouter_base_url", ""); basis != "" {
		c = c.WithBaseURL(basis)
	}
	return c, nil
}
