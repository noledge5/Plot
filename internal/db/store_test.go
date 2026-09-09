package db

import "testing"

func testDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// Der Baum ist das Fundament: eine Zeitlinie darf niemals Inhalte aus einer
// verworfenen Fassung sehen.
func TestPfadUndZweige(t *testing.T) {
	d := testDB(t)
	s, err := d.CreateStory("Testgeschichte", "Du bist der Spielleiter.")
	if err != nil {
		t.Fatalf("CreateStory: %v", err)
	}

	frage, err := d.AddNode(s.ID, nil, "turn", "user", "Ich klopfe an.", "", "")
	if err != nil {
		t.Fatalf("AddNode Wurzel: %v", err)
	}
	antwortA, err := d.AddNode(s.ID, &frage.ID, "turn", "assistant", "Niemand öffnet.", "modell-a", "")
	if err != nil {
		t.Fatalf("AddNode A: %v", err)
	}
	// Regenerieren: zweites Kind am selben Elternknoten, A bleibt bestehen.
	antwortB, err := d.AddNode(s.ID, &frage.ID, "turn", "assistant", "Mira öffnet sofort.", "modell-b", "")
	if err != nil {
		t.Fatalf("AddNode B: %v", err)
	}

	pfad, err := d.Path(antwortA.ID)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if len(pfad) != 2 {
		t.Fatalf("Pfadlänge %d, erwartet 2", len(pfad))
	}
	if pfad[0].ID != frage.ID || pfad[1].ID != antwortA.ID {
		t.Fatalf("Pfad in falscher Reihenfolge: %v, %v", pfad[0].ID, pfad[1].ID)
	}
	for _, n := range pfad {
		if n.ID == antwortB.ID {
			t.Fatal("verworfene Fassung taucht im Pfad der anderen auf")
		}
	}

	alt, err := d.Alternatives(antwortA.ID)
	if err != nil {
		t.Fatalf("Alternatives: %v", err)
	}
	if len(alt) != 2 {
		t.Fatalf("Alternativen %d, erwartet 2", len(alt))
	}

	// Ein neuer Zug hinter B verlängert nur B.
	weiter, err := d.AddNode(s.ID, &antwortB.ID, "turn", "user", "Ich trete ein.", "", "")
	if err != nil {
		t.Fatalf("AddNode weiter: %v", err)
	}
	pfadB, err := d.Path(weiter.ID)
	if err != nil {
		t.Fatalf("Path B: %v", err)
	}
	if len(pfadB) != 3 || pfadB[1].ID != antwortB.ID {
		t.Fatalf("Zweig B falsch: Länge %d", len(pfadB))
	}
}

// Ein Speicherstand ist nur ein Zeiger. Laden heißt: Zeiger setzen.
func TestSpeicherstaendeSindZeiger(t *testing.T) {
	d := testDB(t)
	s, _ := d.CreateStory("Slots", "")
	erst, _ := d.AddNode(s.ID, nil, "turn", "user", "Anfang", "", "")
	zweit, _ := d.AddNode(s.ID, &erst.ID, "turn", "assistant", "Fortsetzung", "m", "")

	slot, err := d.CreateSlot(s.ID, erst.ID, "Vor der Tür", "manual", "Anfang")
	if err != nil {
		t.Fatalf("CreateSlot: %v", err)
	}

	// Head steht nach dem letzten Zug auf zweit.
	st, _ := d.Story(s.ID)
	if st.HeadNodeID == nil || *st.HeadNodeID != zweit.ID {
		t.Fatalf("Head steht falsch")
	}

	// Speicherstand laden.
	if err := d.SetHead(s.ID, &slot.NodeID); err != nil {
		t.Fatalf("SetHead: %v", err)
	}
	st, _ = d.Story(s.ID)
	if *st.HeadNodeID != erst.ID {
		t.Fatalf("Laden hat den Head nicht gesetzt")
	}
	// Nichts wurde gelöscht: der spätere Zug existiert weiter.
	if _, err := d.Node(zweit.ID); err != nil {
		t.Fatalf("Laden hat Inhalte verloren: %v", err)
	}
}

func TestAutoSlotsRotieren(t *testing.T) {
	d := testDB(t)
	s, _ := d.CreateStory("Rotation", "")
	n, _ := d.AddNode(s.ID, nil, "turn", "user", "x", "", "")
	for i := 0; i < 5; i++ {
		if _, err := d.CreateSlot(s.ID, n.ID, "auto", "auto", ""); err != nil {
			t.Fatalf("CreateSlot: %v", err)
		}
	}
	if _, err := d.CreateSlot(s.ID, n.ID, "Wichtig", "manual", ""); err != nil {
		t.Fatalf("manueller Slot: %v", err)
	}
	if err := d.PruneAutoSlots(s.ID, 3); err != nil {
		t.Fatalf("PruneAutoSlots: %v", err)
	}
	slots, _ := d.Slots(s.ID)
	autos, manuell := 0, 0
	for _, sl := range slots {
		if sl.Kind == "auto" {
			autos++
		} else {
			manuell++
		}
	}
	if autos != 3 {
		t.Fatalf("automatische Slots = %d, erwartet 3", autos)
	}
	if manuell != 1 {
		t.Fatal("Rotation hat einen manuellen Speicherstand gelöscht")
	}
}

func TestKostenSummieren(t *testing.T) {
	d := testDB(t)
	s, _ := d.CreateStory("Kosten", "")
	n, _ := d.AddNode(s.ID, nil, "turn", "assistant", "x", "m", "")
	if _, err := d.LogRun(s.ID, &n.ID, "narrate", "m", "{}", "{}", 0.0012, 800, ""); err != nil {
		t.Fatalf("LogRun: %v", err)
	}
	if _, err := d.LogRun(s.ID, &n.ID, "narrate", "m", "{}", "{}", 0.0007, 600, ""); err != nil {
		t.Fatalf("LogRun 2: %v", err)
	}
	total, calls, err := d.Costs(s.ID)
	if err != nil {
		t.Fatalf("Costs: %v", err)
	}
	if calls != 2 {
		t.Fatalf("Aufrufe = %d", calls)
	}
	if total < 0.0018 || total > 0.0020 {
		t.Fatalf("Summe = %v", total)
	}
	// Der Inspektor zeigt immer den jüngsten Lauf.
	r, err := d.RunForNode(n.ID)
	if err != nil {
		t.Fatalf("RunForNode: %v", err)
	}
	if r.CostUSD != 0.0007 {
		t.Fatalf("jüngster Lauf falsch: %v", r.CostUSD)
	}
}
