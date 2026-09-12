package db

import "testing"

// Die Bibliothek führt je Name und Rolle genau eine Fassung. Zwei Fassungen
// derselben Person laufen auseinander - genau das soll sie verhindern.
func TestMerkeFigurUeberschreibtStattZuDoppeln(t *testing.T) {
	d := testDB(t)

	if _, err := d.MerkeFigur("Mira", "npc", Sheet{Kern: "Nachbarin"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.MerkeFigur("Mira", "npc", Sheet{Kern: "Nachbarin, Tierärztin"}); err != nil {
		t.Fatal(err)
	}

	alle, err := d.Bibliothek()
	if err != nil {
		t.Fatal(err)
	}
	if len(alle) != 1 {
		t.Fatalf("Einträge = %d, erwartet 1", len(alle))
	}
	if alle[0].Sheet.Kern != "Nachbarin, Tierärztin" {
		t.Fatalf("Kern = %q, erwartet den neueren Stand", alle[0].Sheet.Kern)
	}
}

// Dieselbe Person als Spielerfigur und als NPC sind zwei Blätter.
func TestRolleTrenntEintraege(t *testing.T) {
	d := testDB(t)
	if _, err := d.MerkeFigur("Jonas", "npc", Sheet{Kern: "der Bruder"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.MerkeFigur("Jonas", "persona", Sheet{Kern: "ich"}); err != nil {
		t.Fatal(err)
	}
	alle, _ := d.Bibliothek()
	if len(alle) != 2 {
		t.Fatalf("Einträge = %d, erwartet 2", len(alle))
	}
}

// Das Blatt wird beim Übernehmen kopiert. Was die Figur in einer Geschichte
// erlebt, darf nicht in eine andere durchschlagen.
func TestUebernommenesBlattIstEineKopie(t *testing.T) {
	d := testDB(t)
	v, err := d.MerkeFigur("Mira", "npc", Sheet{Kern: "Nachbarin", HardLimits: []string{"lügt nie"}})
	if err != nil {
		t.Fatal(err)
	}
	st, _ := d.CreateStory("Eine Geschichte", "prompt")
	c, err := d.CreateCharacter(st.ID, v.Rolle, v.Name, v.Sheet)
	if err != nil {
		t.Fatal(err)
	}

	geaendert := c.Sheet
	geaendert.Kern = "Nachbarin, inzwischen weggezogen"
	if err := d.UpdateCharacter(c.ID, c.Name, geaendert, true, 0); err != nil {
		t.Fatal(err)
	}

	frisch, err := d.FigurVorlage(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if frisch.Sheet.Kern != "Nachbarin" {
		t.Fatalf("Bibliothek hat sich mitgeändert: %q", frisch.Sheet.Kern)
	}
}

func TestNamenloseFigurWirdAbgelehnt(t *testing.T) {
	d := testDB(t)
	if _, err := d.MerkeFigur("   ", "npc", Sheet{}); err == nil {
		t.Fatal("eine Figur ohne Namen wurde angenommen")
	}
}
