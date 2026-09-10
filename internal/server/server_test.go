package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/noledge5/plot/internal/config"
	"github.com/noledge5/plot/internal/db"
)

// aufbau liefert einen angemeldeten Server samt gefälschtem OpenRouter.
func aufbau(t *testing.T, antwort func(w http.ResponseWriter, r *http.Request)) (*Server, *http.Cookie) {
	t.Helper()
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if antwort != nil {
		fake := httptest.NewServer(http.HandlerFunc(antwort))
		t.Cleanup(fake.Close)
		d.SetSetting("openrouter_base_url", fake.URL)
		d.SetSetting("openrouter_key", "sk-test")
	}

	s := New(&config.Config{DataDir: t.TempDir()}, d,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, jsonReq(t, "POST", "/api/setup", map[string]any{"Passwort": "geheimnis1"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("Einrichtung fehlgeschlagen: %d %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return s, c
		}
	}
	t.Fatal("keine Sitzung erhalten")
	return nil, nil
}

func jsonReq(t *testing.T, method, pfad string, body any) *http.Request {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, pfad, r)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func ruf(t *testing.T, s *Server, c *http.Cookie, method, pfad string, body any) *httptest.ResponseRecorder {
	t.Helper()
	req := jsonReq(t, method, pfad, body)
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestGeschuetzteRoutenBrauchenAnmeldung(t *testing.T) {
	s, _ := aufbau(t, nil)
	if rec := ruf(t, s, nil, "GET", "/api/stories", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Cookie erreichbar: %d", rec.Code)
	}
	// Der Zustand ist absichtlich offen, sonst kommt niemand zum Anmeldedialog.
	if rec := ruf(t, s, nil, "GET", "/api/state", nil); rec.Code != http.StatusOK {
		t.Fatalf("Zustand nicht abrufbar: %d", rec.Code)
	}
}

func TestFalschesPasswortWirdAbgelehnt(t *testing.T) {
	s, _ := aufbau(t, nil)
	rec := ruf(t, s, nil, "POST", "/api/login", map[string]any{"Passwort": "danebengetippt"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("falsches Passwort akzeptiert: %d", rec.Code)
	}
	rec = ruf(t, s, nil, "POST", "/api/login", map[string]any{"Passwort": "geheimnis1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("richtiges Passwort abgelehnt: %d %s", rec.Code, rec.Body)
	}
}

// Der Schlüssel darf die Oberfläche nie erreichen - nur die Auskunft, ob einer
// hinterlegt ist.
func TestSchluesselWirdNichtAusgeliefert(t *testing.T) {
	s, c := aufbau(t, nil)
	s.db.SetSetting("openrouter_key", "sk-supergeheim")
	rec := ruf(t, s, c, "GET", "/api/settings", nil)
	if strings.Contains(rec.Body.String(), "sk-supergeheim") {
		t.Fatal("der Schlüssel steht in der Antwort")
	}
	if !strings.Contains(rec.Body.String(), `"keyGesetzt":true`) {
		t.Fatalf("keyGesetzt fehlt: %s", rec.Body)
	}
}

// fakeStream antwortet wie OpenRouter. Auswertungsaufrufe erkennt es am
// response_format und liefert dann Befunde statt Prosa - sonst scheitert der
// Detektor an nicht-JSON und der Test prüfte nur den Fehlerpfad.
func fakeStream(text string) func(http.ResponseWriter, *http.Request) {
	return fakeStreamMitBefunden(text, nil)
}

func fakeStreamMitBefunden(text string, befunde []map[string]any) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var anfrage map[string]any
		if body, err := io.ReadAll(r.Body); err == nil {
			json.Unmarshal(body, &anfrage)
		}
		if _, auswertung := anfrage["response_format"]; auswertung {
			if befunde == nil {
				befunde = []map[string]any{}
			}
			nutz, _ := json.Marshal(map[string]any{"befunde": befunde})
			w.Header().Set("Content-Type", "text/event-stream")
			b, _ := json.Marshal(map[string]any{
				"model":   "analyst",
				"choices": []any{map[string]any{"delta": map[string]any{"content": string(nutz)}}},
			})
			io.WriteString(w, "data: "+string(b)+"\n\n")
			io.WriteString(w, `data: {"usage":{"prompt_tokens":40,"completion_tokens":10,"total_tokens":50,"cost":0.00001}}`+"\n\n")
			io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, wort := range strings.SplitAfter(text, " ") {
			b, _ := json.Marshal(map[string]any{
				"provider": "Venice",
				"model":    "testmodell",
				"choices":  []any{map[string]any{"delta": map[string]any{"content": wort}}},
			})
			io.WriteString(w, "data: "+string(b)+"\n\n")
		}
		io.WriteString(w, `data: {"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,"cost":0.0004}}`+"\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}
}

func TestZugStreamtUndSpeichert(t *testing.T) {
	s, c := aufbau(t, fakeStream("Mira sah nicht auf."))

	rec := ruf(t, s, c, "POST", "/api/stories", map[string]any{"titel": "Probe"})
	var story db.Story
	json.Unmarshal(rec.Body.Bytes(), &story)

	rec = ruf(t, s, c, "POST", "/api/stories/"+itoa(story.ID)+"/turn",
		map[string]any{"text": "Ich setze mich zu ihr."})
	if rec.Code != http.StatusOK {
		t.Fatalf("Zug fehlgeschlagen: %d %s", rec.Code, rec.Body)
	}
	koerper := rec.Body.String()
	if !strings.Contains(koerper, "event: delta") {
		t.Fatalf("kein Streaming: %s", koerper)
	}
	if !strings.Contains(koerper, "event: fertig") {
		t.Fatalf("kein Abschluss: %s", koerper)
	}

	// Verlauf: Eingabe und Antwort stehen in der Zeitlinie.
	rec = ruf(t, s, c, "GET", "/api/stories/"+itoa(story.ID)+"/path", nil)
	var pfad struct {
		Knoten []db.Node `json:"knoten"`
		Kosten float64   `json:"kosten"`
	}
	json.Unmarshal(rec.Body.Bytes(), &pfad)
	if len(pfad.Knoten) != 2 {
		t.Fatalf("Zeitlinie hat %d Knoten, erwartet 2", len(pfad.Knoten))
	}
	if pfad.Knoten[0].Role != "user" || pfad.Knoten[1].Role != "assistant" {
		t.Fatalf("Rollen falsch: %v", pfad.Knoten)
	}
	if pfad.Knoten[1].Content != "Mira sah nicht auf." {
		t.Fatalf("Antworttext falsch: %q", pfad.Knoten[1].Content)
	}
	// Erzählung und Prüfung sind zwei Aufrufe, beide gehören in die Rechnung.
	if pfad.Kosten < 0.0004 {
		t.Fatalf("Kosten nicht erfasst: %v", pfad.Kosten)
	}
	if !strings.Contains(koerper, "event: befunde") {
		t.Fatalf("Detektor hat nichts nachgereicht:\n%s", koerper)
	}

	// Der Inspektor zeigt den exakten Anfragekörper samt System-Prompt.
	rec = ruf(t, s, c, "GET", "/api/nodes/"+itoa(pfad.Knoten[1].ID)+"/inspect", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Inspektor: %d %s", rec.Code, rec.Body)
	}
	var inspektion struct {
		Erzaehlung db.RunLog   `json:"erzaehlung"`
		Alle       []db.RunLog `json:"alle"`
	}
	json.Unmarshal(rec.Body.Bytes(), &inspektion)
	run := inspektion.Erzaehlung
	if !strings.Contains(run.Request, "Spielleiter") {
		t.Fatalf("System-Prompt fehlt im Protokoll: %s", run.Request)
	}
	if !strings.Contains(run.Request, "Ich setze mich zu ihr.") {
		t.Fatal("Eingabe fehlt im Protokoll")
	}
	if run.Purpose != "narrate" {
		t.Fatalf("Inspektor zeigt den falschen Aufruf: %q", run.Purpose)
	}
	// Die Prüfung hängt am selben Knoten und soll nachlesbar sein.
	var hatDetektor bool
	for _, r := range inspektion.Alle {
		if r.Purpose == "detektor" {
			hatDetektor = true
		}
	}
	if !hatDetektor {
		t.Fatal("Prüfaufruf fehlt in der Inspektion")
	}
}

// Wiederholen darf die vorige Fassung nicht überschreiben.
func TestWiederholenErzeugtGeschwister(t *testing.T) {
	s, c := aufbau(t, fakeStream("Sie schwieg."))
	rec := ruf(t, s, c, "POST", "/api/stories", map[string]any{"titel": "Zweige"})
	var story db.Story
	json.Unmarshal(rec.Body.Bytes(), &story)

	ruf(t, s, c, "POST", "/api/stories/"+itoa(story.ID)+"/turn", map[string]any{"text": "Hallo?"})
	rec = ruf(t, s, c, "GET", "/api/stories/"+itoa(story.ID)+"/path", nil)
	var pfad struct {
		Knoten []db.Node `json:"knoten"`
	}
	json.Unmarshal(rec.Body.Bytes(), &pfad)
	erste := pfad.Knoten[1].ID

	rec = ruf(t, s, c, "POST", "/api/stories/"+itoa(story.ID)+"/regenerate", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("Wiederholen: %d %s", rec.Code, rec.Body)
	}
	rec = ruf(t, s, c, "GET", "/api/nodes/"+itoa(erste)+"/alternatives", nil)
	var alt []db.Node
	json.Unmarshal(rec.Body.Bytes(), &alt)
	if len(alt) != 2 {
		t.Fatalf("Fassungen = %d, erwartet 2", len(alt))
	}
	// Die alte Fassung existiert unverändert weiter.
	if _, err := s.db.Node(erste); err != nil {
		t.Fatalf("erste Fassung verloren: %v", err)
	}
}

func TestSpeicherstandLadenSetztPosition(t *testing.T) {
	s, c := aufbau(t, fakeStream("Text."))
	rec := ruf(t, s, c, "POST", "/api/stories", map[string]any{"titel": "Slots"})
	var story db.Story
	json.Unmarshal(rec.Body.Bytes(), &story)

	ruf(t, s, c, "POST", "/api/stories/"+itoa(story.ID)+"/turn", map[string]any{"text": "eins"})
	rec = ruf(t, s, c, "POST", "/api/stories/"+itoa(story.ID)+"/slots", map[string]any{"name": "Hier"})
	var slot db.Slot
	json.Unmarshal(rec.Body.Bytes(), &slot)
	if slot.Preview == "" {
		t.Fatal("Vorschau fehlt")
	}

	ruf(t, s, c, "POST", "/api/stories/"+itoa(story.ID)+"/turn", map[string]any{"text": "zwei"})
	st, _ := s.db.Story(story.ID)
	if *st.HeadNodeID == slot.NodeID {
		t.Fatal("Position hat sich nicht bewegt")
	}

	rec = ruf(t, s, c, "POST", "/api/slots/"+itoa(slot.ID)+"/load", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Laden: %d %s", rec.Code, rec.Body)
	}
	st, _ = s.db.Story(story.ID)
	if *st.HeadNodeID != slot.NodeID {
		t.Fatal("Laden hat die Position nicht gesetzt")
	}
	// Die verlassene Fortsetzung ist nicht gelöscht, nur nicht mehr im Pfad.
	var anzahl int
	s.db.QueryRow(`SELECT count(*) FROM node WHERE story_id = ?`, story.ID).Scan(&anzahl)
	if anzahl != 4 {
		t.Fatalf("Knotenzahl = %d, erwartet 4 (nichts darf gelöscht werden)", anzahl)
	}
}

// Ohne Schlüssel muss ein klarer Hinweis kommen, kein Serverfehler.
func TestOhneSchluesselKlareMeldung(t *testing.T) {
	s, c := aufbau(t, nil)
	rec := ruf(t, s, c, "POST", "/api/stories", map[string]any{"titel": "x"})
	var story db.Story
	json.Unmarshal(rec.Body.Bytes(), &story)
	rec = ruf(t, s, c, "POST", "/api/stories/"+itoa(story.ID)+"/turn", map[string]any{"text": "hallo"})
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("Status = %d, erwartet 428", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Schlüssel") {
		t.Fatalf("Meldung unbrauchbar: %s", rec.Body)
	}
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// Der Detektor markiert, statt zu verwerfen: der Text bleibt stehen, der
// Befund hängt daran.
func TestDetektorMeldetBefund(t *testing.T) {
	s, c := aufbau(t, fakeStreamMitBefunden("Sie nickte sofort.", []map[string]any{
		{"muster": 1, "zitat": "Sie nickte sofort.", "grund": "Zustimmung ohne jeden Preis."},
		{"muster": 99, "zitat": "erfunden", "grund": "gibt es nicht"},
	}))
	rec := ruf(t, s, c, "POST", "/api/stories", map[string]any{"titel": "Befunde"})
	var story db.Story
	json.Unmarshal(rec.Body.Bytes(), &story)

	rec = ruf(t, s, c, "POST", "/api/stories/"+itoa(story.ID)+"/turn", map[string]any{"text": "Kommst du mit?"})
	if !strings.Contains(rec.Body.String(), "event: befunde") {
		t.Fatalf("kein Befund-Ereignis:\n%s", rec.Body)
	}

	rec = ruf(t, s, c, "GET", "/api/stories/"+itoa(story.ID)+"/path", nil)
	var pfad struct {
		Knoten []db.Node `json:"knoten"`
	}
	json.Unmarshal(rec.Body.Bytes(), &pfad)
	antwort := pfad.Knoten[len(pfad.Knoten)-1]

	var flags Flags
	if err := json.Unmarshal([]byte(antwort.FlagsJSON), &flags); err != nil {
		t.Fatalf("Befunde nicht am Knoten: %v (%q)", err, antwort.FlagsJSON)
	}
	if !flags.Geprueft {
		t.Fatal("Knoten nicht als geprüft markiert")
	}
	// Muster 99 gibt es nicht - erfundene Nummern dürfen nicht durchrutschen.
	if len(flags.Befunde) != 1 {
		t.Fatalf("Befunde = %d, erwartet 1: %+v", len(flags.Befunde), flags.Befunde)
	}
	if flags.Befunde[0].Kurz != "Zustimmung ohne Preis" {
		t.Fatalf("Muster nicht benannt: %+v", flags.Befunde[0])
	}
	// Der Text selbst bleibt unangetastet.
	if antwort.Content != "Sie nickte sofort." {
		t.Fatalf("Text verändert: %q", antwort.Content)
	}
}

// Abgeschaltet heißt abgeschaltet: kein Aufruf, keine Kosten.
func TestDetektorAbschaltbar(t *testing.T) {
	s, c := aufbau(t, fakeStream("Text."))
	set := s.settings()
	set.DetektorAn = false
	roh, _ := json.Marshal(set)
	s.db.SetSetting("app_settings", string(roh))

	rec := ruf(t, s, c, "POST", "/api/stories", map[string]any{"titel": "Aus"})
	var story db.Story
	json.Unmarshal(rec.Body.Bytes(), &story)
	rec = ruf(t, s, c, "POST", "/api/stories/"+itoa(story.ID)+"/turn", map[string]any{"text": "hallo"})
	if strings.Contains(rec.Body.String(), "event: befunde") {
		t.Fatal("Detektor lief trotz Abschaltung")
	}
	var anzahl int
	s.db.QueryRow(`SELECT count(*) FROM run_log WHERE purpose = 'detektor'`).Scan(&anzahl)
	if anzahl != 0 {
		t.Fatalf("Detektor-Aufrufe = %d, erwartet 0", anzahl)
	}
}
