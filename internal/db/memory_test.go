package db

import "testing"

func TestFaktenSuchen(t *testing.T) {
	d := testDB(t)
	s, _ := d.CreateStory("Gedächtnis", "")

	for _, f := range []Fact{
		{StoryID: s.ID, Betrifft: "Mira", Text: "Mira hat einen Bruder in Kessin."},
		{StoryID: s.ID, Betrifft: "Mira", Text: "Die Pacht ist bis Freitag fällig."},
		{StoryID: s.ID, Betrifft: "Jonas", Text: "Jonas trägt den Ring seines Vaters."},
	} {
		if _, err := d.CreateFact(f); err != nil {
			t.Fatalf("CreateFact: %v", err)
		}
	}

	treffer, err := d.SucheFakten(s.ID, "Was weißt du über den Bruder?", 10)
	if err != nil {
		t.Fatalf("SucheFakten: %v", err)
	}
	if len(treffer) != 1 || treffer[0].Betrifft != "Mira" {
		t.Fatalf("Treffer falsch: %+v", treffer)
	}

	// Eigennamen sind der häufigste Suchfall im Rollenspiel.
	treffer, _ = d.SucheFakten(s.ID, "Jonas kommt herein", 10)
	if len(treffer) != 1 || treffer[0].Betrifft != "Jonas" {
		t.Fatalf("Namenssuche fehlgeschlagen: %+v", treffer)
	}

	// Zeichen aus dem Erzähltext dürfen die Suche nicht zum Fehler bringen -
	// FTS5 hat eine eigene Abfragesprache.
	for _, heikel := range []string{`"Gib mir den Brief!"`, "Er sagte: 'nein'.", "— und dann?", "(Mira)"} {
		if _, err := d.SucheFakten(s.ID, heikel, 5); err != nil {
			t.Fatalf("Suche scheiterte an %q: %v", heikel, err)
		}
	}
	// Zu kurze Wörter ergeben keine sinnvolle Suche, dürfen aber nicht stören.
	if treffer, err := d.SucheFakten(s.ID, "ob es", 5); err != nil || len(treffer) != 0 {
		t.Fatalf("kurze Wörter: %v / %+v", err, treffer)
	}
}

// Ein Fakt wird außer Kraft gesetzt, nicht gelöscht: so bleibt darstellbar,
// dass etwas einmal galt.
func TestFaktZurueckziehen(t *testing.T) {
	d := testDB(t)
	s, _ := d.CreateStory("Wandel", "")
	n, _ := d.AddNode(s.ID, nil, "turn", "user", "x", "", "")

	f, err := d.CreateFact(Fact{StoryID: s.ID, Betrifft: "Mira", Text: "Mira ist Ärztin."})
	if err != nil {
		t.Fatalf("CreateFact: %v", err)
	}
	if err := d.Zurueckziehen(f.ID, n.ID); err != nil {
		t.Fatalf("Zurueckziehen: %v", err)
	}

	nachher, _ := d.Fact(f.ID)
	if nachher.Status != "retired" {
		t.Fatalf("Status = %q", nachher.Status)
	}
	if nachher.GiltBis == nil || *nachher.GiltBis != n.ID {
		t.Fatal("Gültigkeitsende nicht gesetzt")
	}
	// Zurückgezogene Fakten tauchen in der Suche nicht mehr auf.
	if treffer, _ := d.SucheFakten(s.ID, "Ärztin", 10); len(treffer) != 0 {
		t.Fatalf("zurückgezogener Fakt wird noch gefunden: %+v", treffer)
	}
	// Der Text bleibt aber lesbar.
	if nachher.Text == "" {
		t.Fatal("Text verloren")
	}
}

func TestChronikWaechst(t *testing.T) {
	d := testDB(t)
	s, _ := d.CreateStory("Chronik", "")
	a, _ := d.AddNode(s.ID, nil, "turn", "user", "eins", "", "")
	b, _ := d.AddNode(s.ID, &a.ID, "turn", "assistant", "zwei", "m", "")

	if bis, _ := d.LetzteZusammengefasst(s.ID); bis != 0 {
		t.Fatalf("leere Chronik meldet %d", bis)
	}
	if _, err := d.CreateSummary(s.ID, 1, a.ID, b.ID, "Sie trafen sich."); err != nil {
		t.Fatalf("CreateSummary: %v", err)
	}
	bis, _ := d.LetzteZusammengefasst(s.ID)
	if bis != b.ID {
		t.Fatalf("Chronik reicht bis %d, erwartet %d", bis, b.ID)
	}
	alle, _ := d.Summaries(s.ID)
	if len(alle) != 1 || alle[0].Text != "Sie trafen sich." {
		t.Fatalf("Chronik falsch: %+v", alle)
	}
}
