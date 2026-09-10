# Plot

Lokale Rollenspiel-Engine für die eigene NAS: Erzählung über die OpenRouter-API,
mit eigenem System-Prompt, Beziehungsmechanik, dauerhaftem Gedächtnis und
Speicherständen. Ein Binary, eine SQLite-Datei, erreichbar von jedem Gerät im
Tailnet.

**Stand: M0 und M1 sind fertig.** Spielbar mit Streaming, Turn-Baum,
Speicherständen, Payload-Inspektor, Kostenanzeige und Modellvergleich.
Charakterkarten, Gedächtnis und die Autonomie-Schicht folgen in M2–M5.

## Installation auf der Synology

**Mit Container Manager:** Abbild als Datei einspielen (Abbild → Hinzufügen → Von
Datei hinzufügen) und starten, mit Port 8080 und `/volume1/docker/plot` als
`/data`. Wer ein Projekt bevorzugt, nimmt [`docker-compose.lokal.yml`](docker-compose.lokal.yml);
für den Weg über die Registry [`docker-compose.yml`](docker-compose.yml) — dann ist
zu beachten, dass ein neues GHCR-Paket privat ist und einmalig freigegeben werden muss.

**Als Binary, ohne Docker:** `plot-linux-arm64` per File Station nach `/volume1/plot/`
legen und im Aufgabenplaner als Autostart eintragen — mit `chmod +x` als erster
Zeile, weil hochgeladene Dateien nicht ausführbar sind. Spart den Docker-Daemon.

Ausführlich, mit allen Klickpfaden: [docs/BETRIEB-DS124.md](docs/BETRIEB-DS124.md).

Beim ersten Aufruf legst du ein Passwort fest, danach trägst du unter
Einstellungen den OpenRouter-Schlüssel ein. Der bleibt serverseitig und erreicht
die Oberfläche nie.

## Was M1 kann

- **Erzählen mit Streaming.** Text erscheint, während er entsteht; abbrechen
  behält, was schon da ist.
- **Turn-Baum.** Jeder Zug ist ein Knoten. „Nochmal" erzeugt eine zweite Fassung,
  statt die erste zu ersetzen — zwischen den Fassungen wird geblättert.
- **Speicherstände** als benannte Zeiger auf einen Knoten. Laden setzt nur den
  Zeiger; der verlassene Zweig bleibt vollständig erhalten. Alle fünf Züge
  entsteht automatisch einer, mit Rotation.
- **Payload-Inspektor.** „was ging raus" zeigt an jedem Zug den exakten
  Anfragekörper mit System-Prompt, Verbrauch, Kosten und Dauer.
- **Modellvergleich.** Derselbe Prompt an zwei Modelle nebeneinander — die Art,
  ein Modell für deutsche Prosa auszuwählen, die nicht auf Datenblätter baut.
- **Routing-Prüfung.** Ein Aufruf über ein einziges Token je Rolle zeigt, welcher
  Anbieter tatsächlich bedient — oder warum keiner die Bedingungen erfüllt.
- **Kostenanzeige** pro Geschichte, aus dem `usage`-Objekt der Antwort.

## Dokumentation

- [Architektur- und Umsetzungsplan](docs/PLAN.md)
- [Getroffene Entscheidungen](docs/ENTSCHEIDUNGEN.md)
- [Modell-Auswahl](docs/modelle.md)
- [Betrieb auf der DS124](docs/BETRIEB-DS124.md)
- [Offene Fragen](docs/OPEN-QUESTIONS.md)
- [Startvorlage für den System-Prompt](docs/system-prompt-vorlage.md)

## Entwicklung

```bash
cd web && npm install && npm run build   # Oberfläche ins Binary einbetten
cd .. && go test ./...
go run ./cmd/plot --data ./data --addr :8080
```

Ohne vorangegangenen Frontend-Build läuft das Binary trotzdem; es liefert dann
statt der Oberfläche einen Hinweis, die API funktioniert normal. Für die
Frontend-Entwicklung mit heißem Neuladen: `npm run dev` in `web/`, das Backend
daneben auf Port 8080.
