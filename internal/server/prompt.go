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
}

func storySettings(s *db.Story) StorySettings {
	set := StorySettings{DruckSchwelle: 60}
	if strings.TrimSpace(s.SettingsJSON) != "" {
		_ = json.Unmarshal([]byte(s.SettingsJSON), &set)
	}
	if set.DruckSchwelle <= 0 {
		set.DruckSchwelle = 60
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
	direktiven []string, chronik string, fakten []db.Fact) map[string]string {
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
		msgs = append(msgs, openrouter.Message{Role: rolle, Content: n.Content})
	}
	return msgs, abgeschnitten
}
