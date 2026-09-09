package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	sessionCookie = "plot_session"
	sessionDauer  = 30 * 24 * time.Hour
)

// hashPassword erzeugt einen argon2id-Hash im Format $argon2id$v=19$m=,t=,p=$salt$hash.
func hashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	// 64 MB Speicher wären auf einer NAS mit 1 GB RAM unhöflich; 19 MB ist die
	// von der argon2-Spezifikation empfohlene Untergrenze für argon2id.
	const speicherKiB, durchgaenge, parallel = 19 * 1024, 2, 1
	sum := argon2.IDKey([]byte(pw), salt, durchgaenge, speicherKiB, parallel, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		speicherKiB, durchgaenge, parallel,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum)), nil
}

func verifyPassword(pw, encoded string) bool {
	teile := strings.Split(encoded, "$")
	if len(teile) != 6 || teile[1] != "argon2id" {
		return false
	}
	var speicherKiB uint32
	var durchgaenge uint32
	var parallel uint8
	if _, err := fmt.Sscanf(teile[3], "m=%d,t=%d,p=%d", &speicherKiB, &durchgaenge, &parallel); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(teile[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(teile[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, durchgaenge, speicherKiB, parallel, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func token() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// bremse verzögert wiederholte Fehlversuche. Der Dienst hängt im Tailnet und
// ist nicht öffentlich erreichbar, aber ein Passwort ohne jede Bremse wäre
// trotzdem fahrlässig.
type bremse struct {
	mu       sync.Mutex
	versuche int
	bis      time.Time
}

func (b *bremse) erlaubt() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return time.Now().After(b.bis)
}

func (b *bremse) fehlschlag() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.versuche++
	if b.versuche >= 5 {
		b.bis = time.Now().Add(time.Duration(b.versuche-4) * 10 * time.Second)
	}
}

func (b *bremse) erfolg() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.versuche = 0
	b.bis = time.Time{}
}

// eingerichtet sagt, ob beim ersten Start schon ein Passwort gesetzt wurde.
func (s *Server) eingerichtet() bool {
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM users`).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

func (s *Server) angemeldet(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return false
	}
	var ablauf string
	err = s.db.QueryRow(`SELECT expires_at FROM sessions WHERE token = ?`, c.Value).Scan(&ablauf)
	if err != nil {
		return false
	}
	t, err := time.Parse(time.RFC3339, ablauf)
	if err != nil || time.Now().After(t) {
		return false
	}
	return true
}

func (s *Server) sitzungSetzen(w http.ResponseWriter, userID int64) error {
	tok, err := token()
	if err != nil {
		return err
	}
	jetzt := time.Now().UTC()
	_, err = s.db.Exec(`INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		tok, userID, jetzt.Format(time.RFC3339), jetzt.Add(sessionDauer).Format(time.RFC3339))
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Kein Secure-Flag: im Tailnet läuft der Zugriff über HTTP, die
		// Verschlüsselung macht WireGuard. Mit Secure käme das Cookie nie an.
		Expires: jetzt.Add(sessionDauer),
	})
	return nil
}

// requireAuth schützt alles außer den Einrichtungs- und Anmeldepfaden.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.angemeldet(r) {
			writeError(w, http.StatusUnauthorized, "nicht angemeldet")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Handler ---

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"eingerichtet": s.eingerichtet(),
		"angemeldet":   s.angemeldet(r),
		"keyGesetzt":   s.db.Setting("openrouter_key", "") != "",
	})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if s.eingerichtet() {
		writeError(w, http.StatusConflict, "es ist bereits ein Passwort gesetzt")
		return
	}
	var in struct{ Passwort string }
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	if len([]rune(in.Passwort)) < 8 {
		writeError(w, http.StatusBadRequest, "das Passwort braucht mindestens 8 Zeichen")
		return
	}
	hash, err := hashPassword(in.Passwort)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Passwort konnte nicht gesichert werden")
		return
	}
	res, err := s.db.Exec(`INSERT INTO users (name, pwhash, created_at) VALUES (?, ?, ?)`,
		"ich", hash, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Konto konnte nicht angelegt werden")
		return
	}
	id, _ := res.LastInsertId()
	if err := s.sitzungSetzen(w, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Sitzung konnte nicht gestartet werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.bremse.erlaubt() {
		writeError(w, http.StatusTooManyRequests, "zu viele Fehlversuche, bitte kurz warten")
		return
	}
	var in struct{ Passwort string }
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Anfrage nicht lesbar")
		return
	}
	var id int64
	var hash string
	err := s.db.QueryRow(`SELECT id, pwhash FROM users ORDER BY id LIMIT 1`).Scan(&id, &hash)
	if err != nil || !verifyPassword(in.Passwort, hash) {
		s.bremse.fehlschlag()
		writeError(w, http.StatusUnauthorized, "falsches Passwort")
		return
	}
	s.bremse.erfolg()
	if err := s.sitzungSetzen(w, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Sitzung konnte nicht gestartet werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.db.Exec(`DELETE FROM sessions WHERE token = ?`, c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

var errKeinKey = errors.New("kein OpenRouter-Schlüssel hinterlegt")
