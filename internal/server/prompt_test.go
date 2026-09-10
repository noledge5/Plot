package server

import (
	"strings"
	"testing"

	"github.com/noledge5/plot/internal/db"
)

func figur(name string) db.Character {
	return db.Character{Name: name, Rolle: "npc", Aktiv: true, Sheet: db.Sheet{
		Kern:        name + " ist Wirtin.",
		Sprechweise: "kurze Sätze",
		Drives:      []db.Drive{{Ziel: "die Pacht zusammenbekommen", Druck: 80}},
		HardLimits:  []string{"jemanden an die Wache verraten"},
		SoftLimits:  []db.SoftLimit{{Was: "über ihren Bruder reden", ErstAb: map[string]int{"trust": 60}}},
		Secrets:     []db.Secret{{Text: "dass sie den Brief gelesen hat", PreisgabeAb: map[string]int{"trust": 80}}},
	}}
}

func TestPlatzhalterWerdenGefuellt(t *testing.T) {
	st := &db.Story{SystemPrompt: "Du bist Spielleiter.\n\n{{characters}}\n\n{{limits}}\n\n{{drives}}"}
	mira := figur("Mira")
	text, bloecke := baueSystemPrompt(st.SystemPrompt,
		promptWerte(st, StorySettings{DruckSchwelle: 60}, nil, []db.Character{mira}, nil))

	if !strings.Contains(text, "Mira ist Wirtin.") {
		t.Fatalf("Figurenblatt fehlt:\n%s", text)
	}
	if !strings.Contains(text, "jemanden an die Wache verraten") {
		t.Fatalf("harte Grenze fehlt:\n%s", text)
	}
	if len(bloecke) != 3 {
		t.Fatalf("Blöcke = %d, erwartet 3", len(bloecke))
	}
	for _, b := range bloecke {
		if b.Tokens <= 0 {
			t.Fatalf("Block %q ohne Tokenschätzung", b.Platzhalter)
		}
	}
}

// Ein Tippfehler im Platzhalter darf nicht dazu führen, dass stillschweigend
// Text verschwindet - er soll im Editor auffallen.
func TestUnbekannterPlatzhalterBleibtStehen(t *testing.T) {
	st := &db.Story{SystemPrompt: "Hallo {{charaktere}} und {{characters}}"}
	text, _ := baueSystemPrompt(st.SystemPrompt, promptWerte(st, StorySettings{}, nil, nil, nil))
	if !strings.Contains(text, "{{charaktere}}") {
		t.Fatalf("unbekannter Platzhalter wurde entfernt: %q", text)
	}
}

// Schwellen gehören als Verhalten ins Prompt, nicht als Zahl: Modelle
// befolgen "wenn sie dir wirklich vertraut" zuverlässiger als "trust >= 60".
func TestSchwellenWerdenZuVerhalten(t *testing.T) {
	grenzen := rendereGrenzen([]db.Character{figur("Mira")})
	if strings.Contains(grenzen, "60") || strings.Contains(grenzen, "trust") {
		t.Fatalf("Zahlen oder Achsennamen im Prompt:\n%s", grenzen)
	}
	if !strings.Contains(grenzen, "wirklich vertraut") {
		t.Fatalf("Schwelle nicht in Verhalten übersetzt:\n%s", grenzen)
	}
	if !strings.Contains(grenzen, "Mira verschweigt") {
		t.Fatalf("Geheimnis fehlt:\n%s", grenzen)
	}
}

// Über der Druckschwelle wird aus einem Wunsch eine Handlungsanweisung - das
// ist der Unterschied zwischen reagierenden und handelnden Figuren.
func TestAntriebsdruckWirdZurAnweisung(t *testing.T) {
	mira := figur("Mira")
	hoch := rendereAntriebe([]db.Character{mira}, 60)
	if !strings.Contains(hoch, "dringend") || !strings.Contains(hoch, "zur Sprache") {
		t.Fatalf("hoher Druck ohne Anweisung:\n%s", hoch)
	}
	niedrig := rendereAntriebe([]db.Character{mira}, 90)
	if strings.Contains(niedrig, "dringend") {
		t.Fatalf("Druck unter der Schwelle wurde trotzdem dringend:\n%s", niedrig)
	}
	if !strings.Contains(niedrig, "ohne es zu zeigen") {
		t.Fatalf("verdeckter Antrieb fehlt:\n%s", niedrig)
	}
}

// Ohne Kennzeichnung hält das Modell die Stilprobe für bereits geschehene
// Handlung und spinnt sie fort.
func TestStilbeispielWirdAlsProbeGekennzeichnet(t *testing.T) {
	text := rendereStilbeispiel("Der Regen hörte auf, als sie die Tür schloss.")
	if !strings.Contains(text, "Stilprobe") {
		t.Fatalf("nicht als Probe gekennzeichnet:\n%s", text)
	}
	if rendereStilbeispiel("   ") != "" {
		t.Fatal("leeres Stilbeispiel erzeugt Text")
	}
}

// Leere Platzhalter dürfen keine Löcher hinterlassen: drei Leerzeilen sehen
// im Prompt nach Fehler aus und kosten Tokens.
func TestLeereBloeckeHinterlassenKeineLuecken(t *testing.T) {
	st := &db.Story{SystemPrompt: "Anfang\n\n{{characters}}\n\n{{limits}}\n\n{{drives}}\n\nEnde"}
	text, _ := baueSystemPrompt(st.SystemPrompt, promptWerte(st, StorySettings{}, nil, nil, nil))
	if strings.Contains(text, "\n\n\n") {
		t.Fatalf("Lücken im Prompt:\n%q", text)
	}
	if !strings.HasPrefix(text, "Anfang") || !strings.HasSuffix(text, "Ende") {
		t.Fatalf("Text falsch beschnitten:\n%q", text)
	}
}

func TestVerlaufWirdBegrenzt(t *testing.T) {
	pfad := make([]db.Node, 10)
	for i := range pfad {
		pfad[i] = db.Node{Role: "user", Content: "Zug"}
	}
	msgs, abgeschnitten := baueNachrichten("System", pfad, 4)
	if abgeschnitten != 6 {
		t.Fatalf("abgeschnitten = %d, erwartet 6", abgeschnitten)
	}
	if len(msgs) != 5 { // System + 4 Züge
		t.Fatalf("Nachrichten = %d, erwartet 5", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatal("System-Prompt steht nicht vorn")
	}
}
