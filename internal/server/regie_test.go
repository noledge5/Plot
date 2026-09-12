package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/noledge5/plot/internal/db"
)

// erzaehlerFake antwortet auf jeden Erzählaufruf mit demselben Satz und legt
// die letzte Anfrage ab, damit der Test hineinsehen kann.
func erzaehlerFake(letzte *string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var koerper map[string]any
		json.NewDecoder(r.Body).Decode(&koerper)
		if b, err := json.Marshal(koerper); err == nil {
			*letzte = string(b)
		}
		if koerper["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte(`data: {"choices":[{"delta":{"content":"Der Flur bleibt still."}}],"model":"m"}` + "\n\n"))
			w.Write([]byte(`data: {"choices":[{"delta":{}}],"model":"m","provider":"fake","usage":{"total_tokens":9}}` + "\n\n"))
			w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}],"model":"m","provider":"fake"}`))
	}
}

// spielGrundlage legt eine Geschichte mit einem gespielten Zug an.
func spielGrundlage(t *testing.T, s *Server, c *http.Cookie) int64 {
	t.Helper()
	// Analyse-Läufe aus, sonst rauscht der Fake dazwischen.
	ruf(t, s, c, "PUT", "/api/settings", map[string]any{
		"key": "sk-test",
		"settings": map[string]any{
			"narratorModel": "m", "analystModel": "m", "maxTokens": 100,
			"maxHistoryTurns": 20, "temperature": 0.7,
			"detektorAn": false, "gedaechtnisAn": false, "beziehungenAn": false,
		},
	})
	rec := ruf(t, s, c, "POST", "/api/stories", map[string]any{"titel": "Test"})
	var st db.Story
	json.Unmarshal(rec.Body.Bytes(), &st)
	if rec := ruf(t, s, c, "POST", "/api/stories/1/turn",
		map[string]any{"text": "Ich klopfe an.", "modus": "handlung"}); rec.Code != http.StatusOK {
		t.Fatalf("erster Zug: %d %s", rec.Code, rec.Body)
	}
	return st.ID
}

func pfadVon(t *testing.T, s *Server, c *http.Cookie, id int64) []db.Node {
	t.Helper()
	rec := ruf(t, s, c, "GET", "/api/stories/1/path", nil)
	var antwort struct {
		Knoten []db.Node `json:"knoten"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &antwort); err != nil {
		t.Fatalf("Pfad nicht lesbar: %v", err)
	}
	return antwort.Knoten
}

// Der rückwirkende Eingriff muss die falsche Stelle wirklich aus dem Verlauf
// nehmen. Bliebe sie stehen, baute das Modell weiter darauf auf - und genau
// dagegen ist die Korrektur da.
func TestKorrekturNimmtDieLetzteErzaehlungVomPfad(t *testing.T) {
	var letzte string
	s, c := aufbau(t, erzaehlerFake(&letzte))
	id := spielGrundlage(t, s, c)

	vorher := pfadVon(t, s, c, id)
	if len(vorher) != 2 || vorher[1].Role != "assistant" {
		t.Fatalf("Ausgangslage falsch: %+v", vorher)
	}
	verworfen := vorher[1].ID

	rec := ruf(t, s, c, "POST", "/api/stories/1/korrektur",
		map[string]any{"text": "Mira ist deine Freundin, sie öffnet."})
	if rec.Code != http.StatusOK {
		t.Fatalf("Korrektur: %d %s", rec.Code, rec.Body)
	}

	nachher := pfadVon(t, s, c, id)
	if len(nachher) != 3 {
		t.Fatalf("Pfad = %d Knoten, erwartet 3: %+v", len(nachher), nachher)
	}
	for _, n := range nachher {
		if n.ID == verworfen {
			t.Fatal("die verworfene Fassung liegt noch im Pfad")
		}
	}
	if nachher[1].Kind != KindRegie {
		t.Fatalf("die Anweisung fehlt im Verlauf: %+v", nachher[1])
	}
	if nachher[2].Role != "assistant" {
		t.Fatalf("keine neue Erzählung: %+v", nachher[2])
	}
	// Gelöscht wird nichts: Wer sich vertut, soll die alte Fassung noch
	// finden können.
	if _, err := s.db.Node(verworfen); err != nil {
		t.Fatalf("die verworfene Fassung wurde gelöscht: %v", err)
	}
	if !strings.Contains(letzte, "Mira ist deine Freundin") {
		t.Fatal("die Anweisung ging nicht an das Modell")
	}
	if !strings.Contains(letzte, "geht allem anderen vor") {
		t.Fatal("die Anweisung ging ohne Vorrang-Rahmen raus")
	}
}

// Ohne Erzählung gibt es nichts zurückzunehmen.
func TestKorrekturBrauchtEineErzaehlung(t *testing.T) {
	var letzte string
	s, c := aufbau(t, erzaehlerFake(&letzte))
	ruf(t, s, c, "POST", "/api/stories", map[string]any{"titel": "Leer"})
	rec := ruf(t, s, c, "POST", "/api/stories/1/korrektur", map[string]any{"text": "irgendwas"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Code = %d, erwartet 400: %s", rec.Code, rec.Body)
	}
	rec = ruf(t, s, c, "POST", "/api/stories/1/korrektur", map[string]any{"text": "  "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("leere Anweisung angenommen: %d", rec.Code)
	}
}

// Eine stehende Anweisung gilt, bis der Autor sie aufhebt - nicht nur einen Zug.
func TestStehendeRegieGehtBeiJedemZugMit(t *testing.T) {
	var letzte string
	s, c := aufbau(t, erzaehlerFake(&letzte))
	spielGrundlage(t, s, c)

	ruf(t, s, c, "POST", "/api/stories/1/regie-regeln",
		map[string]any{"text": "Mira ist nie abweisend zu dir."})

	for i := 0; i < 3; i++ {
		ruf(t, s, c, "POST", "/api/stories/1/turn",
			map[string]any{"text": "Ich rede weiter.", "modus": "handlung"})
		if !strings.Contains(letzte, "Mira ist nie abweisend") {
			t.Fatalf("Zug %d: die stehende Anweisung fehlt im Prompt", i+1)
		}
	}

	rec := ruf(t, s, c, "DELETE", "/api/stories/1/regie-regeln/0", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Aufheben: %d %s", rec.Code, rec.Body)
	}
	ruf(t, s, c, "POST", "/api/stories/1/turn",
		map[string]any{"text": "Und weiter.", "modus": "handlung"})
	if strings.Contains(letzte, "Mira ist nie abweisend") {
		t.Fatal("die aufgehobene Anweisung geht weiter mit")
	}
}

// Zweimal dasselbe macht den Prompt länger, nicht verbindlicher.
func TestStehendeRegieDoppeltWirdGeschluckt(t *testing.T) {
	s, c := aufbau(t, nil)
	ruf(t, s, c, "POST", "/api/stories", map[string]any{"titel": "Test"})
	ruf(t, s, c, "POST", "/api/stories/1/regie-regeln", map[string]any{"text": "Kein Zeitsprung."})
	rec := ruf(t, s, c, "POST", "/api/stories/1/regie-regeln", map[string]any{"text": "kein zeitsprung."})
	var regeln []string
	json.Unmarshal(rec.Body.Bytes(), &regeln)
	if len(regeln) != 1 {
		t.Fatalf("Regeln = %v, erwartet eine", regeln)
	}
}

// Die Anweisung des Autors steht hinter den Direktiven der Engine: Was zuletzt
// im Prompt steht, wiegt bei kleinen Modellen am schwersten.
func TestAnweisungDesAutorsStehtHinterDerEngine(t *testing.T) {
	var letzte string
	s, c := aufbau(t, erzaehlerFake(&letzte))
	spielGrundlage(t, s, c)
	ruf(t, s, c, "POST", "/api/stories/1/turn",
		map[string]any{"text": "Lass es regnen.", "modus": "regie"})

	zeit := strings.Index(letzte, "Erzähle im Präsens")
	autor := strings.Index(letzte, "geht allem anderen vor")
	if zeit < 0 || autor < 0 {
		t.Fatalf("Direktiven fehlen im Prompt: zeit=%d autor=%d", zeit, autor)
	}
	if autor < zeit {
		t.Fatal("die Anweisung des Autors steht vor der Direktive der Engine")
	}
}

func TestRendereRegie(t *testing.T) {
	if rendereRegie(nil) != "" {
		t.Fatal("leere Liste ergibt einen Block")
	}
	if rendereRegie([]string{"  ", ""}) != "" {
		t.Fatal("nur Leerzeichen ergeben einen Block")
	}
	got := rendereRegie([]string{"eins", "  zwei  "})
	if !strings.HasPrefix(got, regieVorspann) {
		t.Fatalf("kein Vorspann: %q", got)
	}
	if !strings.Contains(got, "- eins\n- zwei") {
		t.Fatalf("Anweisungen nicht als Liste: %q", got)
	}
}
