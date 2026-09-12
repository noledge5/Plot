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
		promptWerte(st, StorySettings{DruckSchwelle: 60}, nil, []db.Character{mira}, nil, "", nil))

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
	text, _ := baueSystemPrompt(st.SystemPrompt, promptWerte(st, StorySettings{}, nil, nil, nil, "", nil))
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
	text, _ := baueSystemPrompt(st.SystemPrompt, promptWerte(st, StorySettings{}, nil, nil, nil, "", nil))
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

// Eine Regieanweisung ist keine Handlung der Figur: Sie darf nicht in der
// Nachrichtenfolge landen, sonst liest das Modell sie als etwas, das der
// Spieler gesagt hat.
func TestRegieBleibtAusDerNachrichtenfolge(t *testing.T) {
	pfad := []db.Node{
		{Kind: "turn", Role: "user", Content: "Ich klopfe an."},
		{Kind: "turn", Role: "assistant", Content: "Niemand öffnet."},
		{Kind: KindRegie, Role: "user", Content: "Lass die Szene enden, ohne dass etwas geklärt wird."},
	}
	msgs, _ := baueNachrichten("System", pfad, 20)
	for _, m := range msgs {
		if strings.Contains(m.Content, "Lass die Szene enden") {
			t.Fatalf("Regieanweisung steht in der Nachrichtenfolge: %+v", m)
		}
	}
	if len(msgs) != 3 { // System + zwei Züge
		t.Fatalf("Nachrichten = %d, erwartet 3", len(msgs))
	}
}

// Sie gilt für den nächsten Zug und danach nicht mehr - sonst sammeln sich
// Anweisungen an, die längst erledigt sind.
func TestRegieGiltNurBisZurNaechstenAntwort(t *testing.T) {
	pfad := []db.Node{
		{Kind: KindRegie, Role: "user", Content: "alte Anweisung"},
		{Kind: "turn", Role: "assistant", Content: "Text."},
		{Kind: KindRegie, Role: "user", Content: "neue Anweisung"},
		{Kind: KindRegie, Role: "user", Content: "und noch eine"},
	}
	offen := offeneRegie(pfad)
	if len(offen) != 2 {
		t.Fatalf("offene Anweisungen = %d, erwartet 2: %v", len(offen), offen)
	}
	if offen[0] != "neue Anweisung" || offen[1] != "und noch eine" {
		t.Fatalf("Reihenfolge oder Inhalt falsch: %v", offen)
	}
	// Nach einer Antwort ist nichts mehr offen.
	if len(offeneRegie(pfad[:2])) != 0 {
		t.Fatal("erledigte Anweisung gilt weiter")
	}
}

// Fakten aus einem verworfenen Zweig dürfen auf dieser Zeitlinie nicht
// gelten - sonst erinnert sich eine Figur an etwas, das nie geschehen ist.
func TestFaktenGeltenNurAufIhremZweig(t *testing.T) {
	aufPfad := map[int64]bool{1: true, 2: true, 5: true}
	ab, weg := int64(2), int64(99)
	nachBis := int64(5)

	faelle := []struct {
		name     string
		fakt     db.Fact
		erwartet bool
	}{
		{"ohne Herkunft gilt immer", db.Fact{Status: "canon"}, true},
		{"Herkunft auf dem Pfad", db.Fact{Status: "canon", GiltAb: &ab}, true},
		{"Herkunft im anderen Zweig", db.Fact{Status: "canon", GiltAb: &weg}, false},
		{"zurückgezogen auf diesem Pfad", db.Fact{Status: "canon", GiltAb: &ab, GiltBis: &nachBis}, false},
		{"zurückgezogen im anderen Zweig", db.Fact{Status: "canon", GiltAb: &ab, GiltBis: &weg}, true},
		{"nur vorgeschlagen", db.Fact{Status: "proposed", GiltAb: &ab}, false},
		{"stillgelegt", db.Fact{Status: "retired", GiltAb: &ab}, false},
	}
	for _, f := range faelle {
		if got := gueltig(f.fakt, aufPfad); got != f.erwartet {
			t.Errorf("%s: gültig = %v, erwartet %v", f.name, got, f.erwartet)
		}
	}
}

// Derselbe Sachverhalt darf nicht bei jedem Zug erneut aufgeschrieben werden.
func TestDoppelteFaktenWerdenErkannt(t *testing.T) {
	vorhanden := []db.Fact{{Text: "Mira hat einen Bruder in Kessin, der beim Zoll arbeitet."}}
	if !istDoppelt("Mira hat einen Bruder in Kessin, welcher beim Zoll arbeitet.", vorhanden) {
		t.Error("Umformulierung nicht als Dopplung erkannt")
	}
	if istDoppelt("Mira schuldet dem Wirt seit dem Frühjahr Geld.", vorhanden) {
		t.Error("anderer Sachverhalt fälschlich als Dopplung verworfen")
	}
}
