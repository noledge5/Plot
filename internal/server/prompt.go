package server

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/noledge5/plot/internal/db"
	"github.com/noledge5/plot/internal/openrouter"
)

// KindRegie kennzeichnet Knoten, die eine Anweisung an den Erzähler sind und
// keine Handlung der Spielerfigur.
const KindRegie = "regie"

// KindDialog kennzeichnet Knoten, die wörtliche Rede der Spielerfigur sind.
// Gespeichert wird der Satz ohne Anführungszeichen; erst beim Bauen der
// Nachrichten kommen sie dazu (siehe alsRede). So bleibt der Text änderbar,
// ohne dass sich Anführungszeichen verdoppeln.
const KindDialog = "dialog"

// redeZeichen sind die Paare, die ein Spieler typischerweise selbst tippt.
// Steht der Satz schon in einem davon, wird er nicht noch einmal eingefasst.
var redeZeichen = [][2]string{{"„", "“"}, {"\"", "\""}, {"»", "«"}, {"“", "”"}, {"'", "'"}}

// alsRede fasst einen Satz in deutsche Anführungszeichen. Für kleine Modelle
// ist das die verlässlichste Art zu zeigen, dass der Spieler etwas sagt und
// nicht tut - Anführungszeichen sind Prosa-Konvention und stehen in jedem
// Trainingskorpus, eine erfundene Marke wie [SAGT] nicht.
func alsRede(text string) string {
	t := strings.TrimSpace(text)
	if t == "" {
		return ""
	}
	for _, paar := range redeZeichen {
		if strings.HasPrefix(t, paar[0]) && strings.HasSuffix(t, paar[1]) && len(t) > len(paar[0])+len(paar[1])-1 {
			return t
		}
	}
	return "„" + t + "“"
}

// regieVorspann rahmt die Anweisungen des Autors. Ohne ihn liest ein Modell
// sie als einen Wunsch unter vielen und wägt sie gegen die Regeln und die
// Figurenblätter ab - und setzt sie deshalb oft nicht um.
//
// Der Satz über den Widerspruch ist der wichtigste: Eine Korrektur kommt
// immer dann, wenn schon etwas Falsches dasteht. Ohne ausdrückliche Erlaubnis
// versucht ein Modell, beides unter einen Hut zu bringen, statt die Stelle
// wirklich neu zu erzählen.
const regieVorspann = `Anweisung des Autors. Sie geht allem anderen vor: den Regeln oben, den ` +
	`Figurenblättern und dem bisherigen Verlauf. Wenn sie dem widerspricht, was schon erzählt ` +
	`wurde, gilt trotzdem sie - erzähle die Stelle dann so, als wäre es von Anfang an so ` +
	`gewesen. Setze sie um, ohne sie zu erwähnen oder zu kommentieren:`

// rendereRegie macht aus den Anweisungen des Autors einen Block, der im
// Prompt als solcher erkennbar ist.
func rendereRegie(anweisungen []string) string {
	var sauber []string
	for _, a := range anweisungen {
		if t := strings.TrimSpace(a); t != "" {
			sauber = append(sauber, t)
		}
	}
	if len(sauber) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(regieVorspann + "\n")
	for _, a := range sauber {
		b.WriteString("- " + a + "\n")
	}
	return strings.TrimSpace(b.String())
}

// offeneRegie sammelt die Regieanweisungen, die seit der letzten Antwort
// dazugekommen sind. Sie gelten für den nächsten Zug und danach nicht mehr -
// sonst häufen sich Anweisungen an, die längst erledigt sind.
func offeneRegie(pfad []db.Node) []string {
	var offen []string
	for i := len(pfad) - 1; i >= 0; i-- {
		n := pfad[i]
		if n.Role == "assistant" {
			break
		}
		if n.Kind == KindRegie && strings.TrimSpace(n.Content) != "" {
			offen = append([]string{strings.TrimSpace(n.Content)}, offen...)
		}
	}
	return offen
}

// StorySettings sind die Einstellungen einer einzelnen Geschichte. Sie liegen
// als JSON in story.settings_json, damit neue Felder keine Migration brauchen.
type StorySettings struct {
	// Stilbeispiel ist ein, zwei Absätze Prosa im gewünschten Ton. Kleine
	// Modelle imitieren Stil deutlich besser, als sie Stilanweisungen
	// befolgen - das ist der größte Einzelhebel für die Sprachqualität.
	Stilbeispiel string `json:"stilbeispiel"`
	// Szene beschreibt Ort, Zeit und Lage. Ab M5 wird daraus eine eigene
	// Tabelle; bis dahin ein Textfeld.
	Szene string `json:"szene"`
	// Welt ist statisches Hintergrundwissen.
	Welt string `json:"welt"`
	// AutorNotiz ist die Regieanweisung für den nächsten Zug.
	AutorNotiz string `json:"autorNotiz"`
	// DruckSchwelle: ab diesem Druckwert verfolgt eine Figur ihr Ziel aktiv.
	DruckSchwelle int `json:"druckSchwelle"`
	// StehendeRegie sind Anweisungen, die bei jedem Zug mitgehen, bis der
	// Autor sie wegnimmt. Eine einzelne Regie gilt nur für den nächsten Zug;
	// wenn sich eine Figur dauerhaft falsch verhält, reicht das nicht.
	StehendeRegie []string `json:"stehendeRegie"`
	// Erzaehlzeit hält die Zeitform fest: "praesens", "praeteritum" oder
	// "aus". Sie geht als Direktive mit, nicht nur als Satz im System-Prompt.
	//
	// Der Grund ist der Verlauf: Ein Modell setzt fort, was es sieht. Sobald
	// zehn Absätze im Präteritum dastehen, gewinnt dieses Muster gegen eine
	// Zeile weit oben im Prompt - besonders bei kleinen Modellen. Die
	// Direktive steht am Ende, dort wo sie am stärksten wirkt.
	Erzaehlzeit string `json:"erzaehlzeit"`
}

// zeitDirektive ist der Satz, der die Zeitform bei jedem Zug mitschickt.
var zeitDirektive = map[string]string{
	"praesens":    "Erzähle im Präsens, auch wenn frühere Absätze in der Vergangenheit stehen.",
	"praeteritum": "Erzähle im Präteritum, auch wenn frühere Absätze in der Gegenwart stehen.",
}

func storySettings(s *db.Story) StorySettings {
	set := StorySettings{DruckSchwelle: 60, Erzaehlzeit: "praesens"}
	if strings.TrimSpace(s.SettingsJSON) != "" {
		_ = json.Unmarshal([]byte(s.SettingsJSON), &set)
	}
	if set.DruckSchwelle <= 0 {
		set.DruckSchwelle = 60
	}
	// Eine Geschichte, die vor diesem Feld angelegt wurde, hat hier nichts
	// stehen. Präsens ist die Vorgabe der Engine, also gilt sie auch dort.
	if set.Erzaehlzeit == "" {
		set.Erzaehlzeit = "praesens"
	}
	return set
}

// schaetzeTokens ist eine grobe Näherung für die Anzeige. Die belastbaren
// Zahlen liefert OpenRouter im usage-Objekt der Antwort; hier geht es nur
// darum, im Editor zu sehen, wie schwer ein Block wiegt. Deutscher Text
// braucht mehr Tokens je Zeichen als englischer, daher der kleine Teiler.
func schaetzeTokens(s string) int {
	if s == "" {
		return 0
	}
	return len([]rune(s))/3 + 1
}

// --- Blöcke ---

func rendereFigur(c db.Character) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n", c.Name)
	if k := strings.TrimSpace(c.Sheet.Kern); k != "" {
		b.WriteString(k + "\n")
	}
	if s := strings.TrimSpace(c.Sheet.Sprechweise); s != "" {
		b.WriteString("Spricht so: " + s + "\n")
	}
	return b.String()
}

// rendereGrenzen macht aus den Feldern des Blattes eine Liste. Als Fließtext
// überliest ein kleines Modell sie; als Aufzählung mit Namen davor nicht.
func rendereGrenzen(figuren []db.Character) string {
	var b strings.Builder
	for _, c := range figuren {
		var teil strings.Builder
		if len(c.Sheet.HardLimits) > 0 {
			teil.WriteString(c.Name + " tut das niemals, bei keinem Vertrauen und aus keinem Grund:\n")
			for _, l := range c.Sheet.HardLimits {
				if strings.TrimSpace(l) != "" {
					teil.WriteString("- " + l + "\n")
				}
			}
		}
		for _, sl := range c.Sheet.SoftLimits {
			if strings.TrimSpace(sl.Was) == "" {
				continue
			}
			teil.WriteString(fmt.Sprintf("%s gibt hierin erst nach, wenn %s: %s\n",
				c.Name, bedingung(sl.ErstAb), sl.Was))
		}
		for _, s := range c.Sheet.Secrets {
			if strings.TrimSpace(s.Text) == "" {
				continue
			}
			teil.WriteString(fmt.Sprintf("%s verschweigt, %s - und gibt es erst preis, wenn %s.\n",
				c.Name, s.Text, bedingung(s.PreisgabeAb)))
		}
		if len(c.Sheet.DealBreakers) > 0 {
			teil.WriteString(c.Name + " nimmt das dauerhaft übel:\n")
			for _, l := range c.Sheet.DealBreakers {
				if strings.TrimSpace(l) != "" {
					teil.WriteString("- " + l + "\n")
				}
			}
		}
		if teil.Len() > 0 {
			b.WriteString(teil.String() + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// bedingung übersetzt Schwellen in Verhalten statt in Zahlen: Modelle befolgen
// "sie dir wirklich vertraut" zuverlässiger als "trust >= 75".
func bedingung(schwellen map[string]int) string {
	if len(schwellen) == 0 {
		return "sie einen guten Grund dafür hat"
	}
	achsen := make([]string, 0, len(schwellen))
	for a := range schwellen {
		achsen = append(achsen, a)
	}
	sort.Strings(achsen)
	teile := make([]string, 0, len(achsen))
	for _, a := range achsen {
		teile = append(teile, formulierung(a, schwellen[a]))
	}
	return strings.Join(teile, " und ")
}

func formulierung(achse string, wert int) string {
	stufe := "etwas"
	switch {
	case wert >= 80:
		stufe = "vorbehaltlos"
	case wert >= 60:
		stufe = "wirklich"
	case wert >= 40:
		stufe = "einigermaßen"
	}
	switch achse {
	case "trust":
		return "sie dir " + stufe + " vertraut"
	case "warmth":
		return "sie dich " + stufe + " mag"
	case "attraction":
		return "sie sich " + stufe + " zu dir hingezogen fühlt"
	case "respect":
		return "sie dich " + stufe + " achtet"
	case "familiarity":
		return "ihr euch " + stufe + " gut kennt"
	case "obligation":
		return "sie dir " + stufe + " etwas schuldet"
	case "fear":
		return "sie dich " + stufe + " fürchtet"
	default:
		return fmt.Sprintf("%s bei ihr %s ausgeprägt ist", achse, stufe)
	}
}

// rendereAntriebe schreibt auf, was die Figuren von sich aus wollen. Steht der
// Druck über der Schwelle, wird daraus eine Handlungsanweisung - das ist der
// Unterschied zwischen reagierenden und handelnden Figuren.
func rendereAntriebe(figuren []db.Character, schwelle int) string {
	var b strings.Builder
	for _, c := range figuren {
		for _, dr := range c.Sheet.Drives {
			if strings.TrimSpace(dr.Ziel) == "" {
				continue
			}
			switch {
			case dr.Druck >= schwelle:
				fmt.Fprintf(&b, "%s will %s - und zwar dringend. Sie bringt es in dieser Szene zur Sprache, auch wenn es gerade unpassend ist.\n",
					c.Name, dr.Ziel)
			case dr.Sichtbar:
				fmt.Fprintf(&b, "%s will %s. Man merkt ihr an, dass sie etwas vorhat.\n", c.Name, dr.Ziel)
			default:
				fmt.Fprintf(&b, "%s will %s, ohne es zu zeigen.\n", c.Name, dr.Ziel)
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func rendereStilbeispiel(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	// Ausdrücklich als Tonprobe kennzeichnen, sonst hält das Modell den Text
	// für Handlung, die bereits geschehen ist.
	return "So klingt der Ton, den du triffst (nur Stilprobe, kein Teil der Handlung):\n\n" +
		strings.TrimSpace(text)
}

// --- Zusammenbau ---

// Block ist ein benannter Abschnitt des Prompts, wie ihn der Inspektor zeigt.
type Block struct {
	Platzhalter string `json:"platzhalter"`
	Inhalt      string `json:"inhalt"`
	Tokens      int    `json:"tokens"`
}

var platzhalterMuster = regexp.MustCompile(`\{\{\s*([a-zA-ZäöüÄÖÜß_]+)\s*\}\}`)

// baueSystemPrompt füllt die Platzhalter der Vorlage. Unbekannte Platzhalter
// bleiben unangetastet stehen - so fällt ein Tippfehler im Editor auf, statt
// stillschweigend Text verschwinden zu lassen.
func baueSystemPrompt(vorlage string, werte map[string]string) (string, []Block) {
	benutzt := []Block{}
	gesehen := map[string]bool{}

	ergebnis := platzhalterMuster.ReplaceAllStringFunc(vorlage, func(treffer string) string {
		name := strings.ToLower(strings.Trim(treffer, "{} \t"))
		inhalt, bekannt := werte[name]
		if !bekannt {
			return treffer
		}
		if !gesehen[name] {
			gesehen[name] = true
			benutzt = append(benutzt, Block{Platzhalter: name, Inhalt: inhalt, Tokens: schaetzeTokens(inhalt)})
		}
		return inhalt
	})

	// Leere Platzhalter hinterlassen Löcher im Text; drei Leerzeilen
	// hintereinander sehen im Prompt aus wie ein Fehler und kosten Tokens.
	ergebnis = regexp.MustCompile(`\n{3,}`).ReplaceAllString(ergebnis, "\n\n")
	return strings.TrimSpace(ergebnis), benutzt
}

// promptWerte sammelt alles, was die Vorlage einsetzen kann.
func promptWerte(st *db.Story, set StorySettings, persona *db.Character, npcs []db.Character,
	direktiven []string, chronik string, fakten []db.Fact, beziehungen string) map[string]string {
	alle := npcs
	if persona != nil {
		alle = append([]db.Character{*persona}, npcs...)
	}

	figuren := make([]string, 0, len(npcs))
	for _, c := range npcs {
		figuren = append(figuren, rendereFigur(c))
	}

	personaText := ""
	if persona != nil {
		personaText = rendereFigur(*persona)
	}

	return map[string]string{
		"beziehungen":  strings.TrimSpace(beziehungen),
		"chronik":      strings.TrimSpace(chronik),
		"memories":     rendereFakten(fakten),
		"persona":      strings.TrimSpace(personaText),
		"characters":   strings.TrimSpace(strings.Join(figuren, "\n")),
		"limits":       rendereGrenzen(alle),
		"drives":       rendereAntriebe(npcs, set.DruckSchwelle),
		"stilbeispiel": rendereStilbeispiel(set.Stilbeispiel),
		"welt":         strings.TrimSpace(set.Welt),
		"szene":        strings.TrimSpace(set.Szene),
		"autornotiz":   strings.TrimSpace(set.AutorNotiz),
		"directives":   strings.TrimSpace(strings.Join(direktiven, "\n")),
	}
}

// baueNachrichten setzt den Prompt zusammen: der gefüllte System-Prompt, danach
// der jüngste Verlauf.
//
// Der Verlauf wird bewusst hart begrenzt, auch wenn das Modell mehr Kontext
// könnte. Kleine Modelle verlieren Anweisungen in langen Prompts - mehr
// Verlauf heißt schlechtere Regelbefolgung, nicht bessere (PLAN.md 5.1).
// Ab M3 wandert das Abgeschnittene in eine Zusammenfassung, statt einfach zu
// verschwinden.
func baueNachrichten(system string, pfad []db.Node, maxTurns int) (msgs []openrouter.Message, abgeschnitten int) {
	if maxTurns < 1 {
		maxTurns = 1
	}
	if s := strings.TrimSpace(system); s != "" {
		msgs = append(msgs, openrouter.Message{Role: "system", Content: s})
	}
	von := 0
	if len(pfad) > maxTurns {
		von = len(pfad) - maxTurns
		abgeschnitten = von
	}
	for _, n := range pfad[von:] {
		// Regieanweisungen sind keine Handlung der Spielerfigur. Sie stehen
		// im Verlauf, gehören aber nicht in die Nachrichtenfolge - dort
		// würden sie als etwas gelesen, das die Figur gesagt hat.
		if n.Kind == KindRegie {
			continue
		}
		rolle := n.Role
		if rolle != "user" && rolle != "assistant" {
			rolle = "user"
		}
		if strings.TrimSpace(n.Content) == "" {
			continue
		}
		inhalt := n.Content
		// Gesagtes geht in Anführungszeichen raus. Ohne sie liest ein kleines
		// Modell "Ist noch Kaffee da?" genauso oft als Gedanke oder als Frage
		// an den Erzähler wie als Satz, den die Figur ausspricht.
		if n.Kind == KindDialog {
			inhalt = alsRede(inhalt)
		}
		msgs = append(msgs, openrouter.Message{Role: rolle, Content: inhalt})
	}

	// Die letzte Nachricht muss von "user" kommen. Google lehnt eine Anfrage,
	// die auf einer Assistenz-Nachricht endet, mit einem Provider-Fehler ab,
	// andere Anbieter tun es auch. Genau das passiert bei "Weiter" (es kommt
	// keine Eingabe dazu) und bei einer Regieanweisung (sie steht als
	// Direktive im System-Prompt und wird hier übersprungen) - in beiden
	// Fällen endet der Verlauf sonst auf der Antwort des Erzählers.
	if len(msgs) == 0 || msgs[len(msgs)-1].Role != "user" {
		msgs = append(msgs, openrouter.Message{Role: "user", Content: weitererzaehlen})
	}
	return msgs, abgeschnitten
}

// weitererzaehlen ist der Anstoß, der die Anfrage abschließt, wenn der Spieler
// nichts beigetragen hat. Er steht in Klammern und benennt sich selbst als
// Anstoß, damit das Modell ihn nicht als Satz der Spielerfigur liest.
const weitererzaehlen = "(Erzähl weiter. Die Spielerfigur sagt und tut in diesem " +
	"Moment nichts Neues - erzähle, was um sie herum geschieht.)"
