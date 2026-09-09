# Getroffene Entscheidungen

Stand: nach der ersten Fragerunde. Was hier steht, ist gesetzt und wird im Plan
nicht mehr als Option geführt.

| # | Entscheidung | Konsequenz |
|---|---|---|
| E1 | **Eigenbau**, kein SillyTavern-Aufsatz | Beziehungs-, Wissens- und Autonomie-Mechanik lassen sich nur so bauen |
| E2 | **Python/FastAPI + React**, du codest nicht mit | Alles muss über die Oberfläche konfigurierbar sein — keine YAML-Handarbeit, kein SSH für den Alltag |
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
