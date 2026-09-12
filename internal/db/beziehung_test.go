package db

import "testing"

func TestBeziehungsstandFaltetNurDenEigenenZweig(t *testing.T) {
	start := map[string]int{"trust": 60, "warmth": 70}
	deltas := []Delta{
		{NodeID: 1, Figur: "Mira", Achse: "trust", Delta: -10},
		{NodeID: 2, Figur: "Mira", Achse: "trust", Delta: -10}, // anderer Zweig
		{NodeID: 3, Figur: "Mira", Achse: "warmth", Delta: 5},
		{NodeID: 3, Figur: "Jonas", Achse: "trust", Delta: -50}, // andere Figur
		{NodeID: 3, Figur: "Mira", Achse: "unsinn", Delta: 99},  // unbekannte Achse
	}
	aufPfad := map[int64]bool{1: true, 3: true}

	stand := Beziehungsstand(start, deltas, "Mira", aufPfad)
	if stand["trust"] != 50 {
		t.Errorf("trust = %d, erwartet 50 (nur der Delta auf dem Pfad)", stand["trust"])
	}
	if stand["warmth"] != 75 {
		t.Errorf("warmth = %d, erwartet 75", stand["warmth"])
	}
	// Achsen ohne Startwert beginnen neutral, nicht ablehnend.
	if stand["respect"] != 0 {
		t.Errorf("respect = %d, erwartet 0", stand["respect"])
	}
	if _, da := stand["unsinn"]; da {
		t.Error("unbekannte Achse wurde übernommen")
	}
}

func TestBeziehungBleibtInGrenzen(t *testing.T) {
	deltas := []Delta{}
	for i := 0; i < 30; i++ {
		deltas = append(deltas, Delta{NodeID: 1, Figur: "Mira", Achse: "trust", Delta: 10})
	}
	stand := Beziehungsstand(map[string]int{"trust": 50}, deltas, "Mira", map[int64]bool{1: true})
	if stand["trust"] != 100 {
		t.Errorf("trust = %d, erwartet Deckel bei 100", stand["trust"])
	}
}
