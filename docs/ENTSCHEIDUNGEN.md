# Getroffene Entscheidungen

Stand: nach der ersten Fragerunde. Was hier steht, ist gesetzt und wird im Plan
nicht mehr als Option geführt.

| # | Entscheidung | Konsequenz |
|---|---|---|
| E1 | **Eigenbau**, kein SillyTavern-Aufsatz | Beziehungs-, Wissens- und Autonomie-Mechanik lassen sich nur so bauen |
| E2 | ~~Python/FastAPI~~ → **Go, ein statisches arm64-Binary** + React | Revidiert nach dem Hardware-Befund (E14). Alles bleibt über die Oberfläche konfigurierbar — kein SSH, keine Konfigdateien von Hand |
| E3 | **Embeddings über OpenRouter** | Ein Key, eine Rechnung, keine CPU-Last auf der NAS; Provider-Filter gilt auch hier |
| E4 | **Slots und Baum**, Baum optional sichtbar | Turn-Baum als Fundament, schlichte Slot-UI darüber |
| E5 | **Spielleiter erzählt alles außer deiner Figur** | Harte Prompt-Regel *und* automatische Nachprüfung, die Verstöße markiert |
| E6 | **Erzählsprache Deutsch** | Modellauswahl wird enger; Modell-Vergleich wird Kernfeature statt Nice-to-have |
| E7 | **Explizite Inhalte erlaubt, Provider gefiltert** | Zero Data Retention erzwungen, Provider-Allowlist pro Story, schneller Modellwechsel mitten im Spiel |
| E8 | **Gruppenszenen mit mehreren NPCs** | Wissensblöcke pro Figur, NPC↔NPC-Beziehungen, Sprecherregie |
| E9 | **In-Game-Zeit wird modelliert** | Zeitstempel an Szenen und Fakten, Beziehungsdrift über Zeit |
| E10 | **Keine Würfel — stattdessen Autonomie-Schicht** | Gefälligkeit wird mechanisch verhindert, nicht erhofft. Siehe PLAN.md §9 |
| E11 | **Keine Off-Screen-Simulation** (vorerst) | Stattdessen Zeitsprung-Zusammenfassung: nur bei explizitem Sprung, ein Call, 80 % des Effekts |
| E12 | **Kleine, günstige Modelle** (Dolphin-Mistral, Gemini-Flash-Klasse) | Kosten fallen als Engpass weg, Regelbefolgung wird zum Engpass. Prompt-Diät, Regelrotation, Direktiven ans Ende — siehe PLAN.md §5.1 und §11 |
| E13 | **Analyst als eigene Modellrolle** | Folge aus E12: Dolphin kann kein striktes JSON-Schema, die Zustandsmechanik braucht es. Erzähler erzählt, Analyst rechnet |
| E14 | **Zielgerät ist die DS124: arm64, 1 GB RAM, Container Manager läuft** | Ausgeliefert wird beides: ein arm64-Image für Container Manager (Hauptweg) und dasselbe Binary zum nativen Start (Alternative, spart den Docker-Daemon). Go statt Python wegen des Arbeitsspeichers, Frontend vorgebaut und eingebettet |
| E15 | **Zugriff über Tailscale, ohne Reverse Proxy** | WireGuard verschlüsselt bereits; kein Zertifikat nötig. App-Login bleibt trotzdem |
| E16 | **ZDR nur für den Analysten, Erzähler über Provider-Allowlist** | Löst den Konflikt aus modelle.md §2: Venice bleibt für die Erzählung erlaubt, alle anderen Anbieter gesperrt |
| E17 | **Retrieval startet mit FTS5, Vektoren erst bei Bedarf** | Folge aus E14: `sqlite-vec` als arm64-Modul ist Mehraufwand, den BM25 auf dieser Hardware vermutlich erspart. Entscheidung fällt in M3 |
| E18 | **Erzählzeit Präsens** | Die Startvorlage erzählt, was gerade geschieht, nicht was geschehen ist. Gilt für neue Geschichten; bestehende holen sie sich im Prompt-Editor über „Vorlage übernehmen" |
| E19 | **Drei Eingabemodi: Handlung, Gesagt, Regie** | Gesagtes geht in deutschen Anführungszeichen an das Modell — Prosa-Konvention statt erfundener Marke, weil kleine Modelle Anführungszeichen aus jedem Trainingskorpus kennen. Regie bleibt aus der Nachrichtenfolge heraus |
| E20 | **Der System-Prompt einer Geschichte wird nie automatisch überschrieben** | Ein Update ändert nur die Vorlage für neue Geschichten. Der Editor sagt, welche Blöcke die Engine füllt, die im eigenen Prompt fehlen, und bietet die aktuelle Vorlage auf Knopfdruck an |
| E21 | **Jede Anfrage endet auf einer Nachricht des Spielers** | Google lehnt eine Anfrage ab, die auf einer Assistenz-Nachricht endet — genau das entstand bei „Weiter" und bei Regie. Endet der Verlauf nicht beim Spieler, hängt die Engine einen Anstoß in Klammern an |
| E22 | **Die Erzählzeit geht als Direktive mit, nicht nur als Satz im Prompt** | Ein Modell setzt fort, was im Verlauf steht. Zehn Absätze Präteritum gewinnen gegen eine Zeile weit oben — die Direktive steht am Ende, wo sie wirkt. Vorgabe Präsens, abschaltbar |
| E23 | **Figurenbibliothek: Grundbeschreibung geschichtsübergreifend, Beziehungsstand nicht** | Das Blatt wird beim Übernehmen kopiert, nicht verknüpft. Was eine Figur in einer Geschichte erlebt, darf nicht in eine andere durchschlagen; ihr Kern soll überall derselbe sein |
| E24 | **Regie ist das Eingriffswerkzeug und schlägt alles andere** | Sie steht als letzter Block im Prompt, mit ausdrücklichem Vorrang vor Regeln, Figurenblättern und Verlauf — samt Erlaubnis, einer schon erzählten Stelle zu widersprechen. Ohne diesen Rahmen wägt ein Modell sie gegen die Regeln ab und setzt sie oft nicht um |
| E25 | **Regie wirkt in drei Reichweiten: nächster Zug, letzter Zug, dauerhaft** | „Ab jetzt" wie bisher. „Letztes neu" nimmt die falsche Erzählung vom Pfad und erzählt sie neu — sonst baut das Modell weiter auf dem Fehler auf. „Dauerhaft" geht bei jedem Zug mit, für Figuren, die sich wiederholt falsch verhalten. Verworfene Fassungen werden nie gelöscht, nur vom Pfad genommen |

## Was du wörtlich gesagt hast, und was ich daraus gemacht habe

> „Geben dir Tendenz alles gelingen zu lassen, brauch ich einen harten Anti-Prompt.
> NPCs müssen möglichst real sein und sich reale Grenzen aufzeigen, sich wehren,
> eigene Antriebe haben."

Das ist der anspruchsvollste Teil des ganzen Projekts, und ein Prompt allein
reicht dafür nicht. Modelle sind auf Zustimmung trainiert; eine Regel wie „sei
nicht gefällig" verliert nach 20 Turns gegen dieses Training — zuverlässig, jedes
Mal. Deshalb bekommt es eine eigene Schicht aus vier Teilen (PLAN.md §9):

1. **Textregeln** im Stilleitfaden — die Basis, aber nur die Basis.
2. **Grenzen als harte Daten** im Charakterblatt statt als Prosa.
3. **Ein Gefälligkeits-Detektor**, der jede Antwort auf sieben konkrete Muster
   prüft und Verstöße markiert.
4. **Ein Widerstandsbudget**, das zählt, wie oft eine Figur schon nachgegeben hat,
   und ab einer Schwelle eine harte Direktive erzwingt.

Punkt 3 und 4 sind der Unterschied zwischen „ich habe es ins Prompt geschrieben"
und „es passiert tatsächlich".

---

## Nachtrag zu E12: was kleine Modelle für die Autonomie-Schicht bedeuten

Kleine Modelle sind **gefälliger** als große, nicht weniger. Sie folgen langen
Regellisten schlechter, verlieren Anweisungen in der Mitte des Prompts und fallen
schneller in Zustimmungsmuster zurück. Deine Entscheidung für die günstige Klasse
macht §9 also nicht überflüssig — sie macht sie zum entscheidenden Teil.

Der Ausgleich kommt aus der Kostenseite: Bei 0,19 Cent pro Turn ist ein
automatischer Neuversuch bei Detektor-Befund praktisch gratis. Was bei einem
teuren Modell eine Abwägung wäre, wird hier zur Standardeinstellung. Die
Autonomie-Schicht kann also öfter eingreifen, als ich ursprünglich geplant hatte —
weil sie es sich leisten kann.
