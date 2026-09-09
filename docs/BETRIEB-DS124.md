# Betrieb auf der DS124

## Der Befund

| | |
|---|---|
| CPU | Realtek RTD1619B, 4 × ARM Cortex-A55, 1,7 GHz (arm64) |
| RAM | 1 GB DDR4, **nicht erweiterbar** |
| DSM | 7.3.2 |
| Container Manager | **läuft** (24.0.2-1606, per Screenshot bestätigt) |

Die offizielle Kompatibilitätsliste führt Realtek-Modelle nicht für Container
Manager, und das Community-Nachrüstprojekt deckt die RTD1619B-Generation nicht ab.
Auf diesem Gerät läuft es trotzdem — die Doku beschreibt also den Docker-Weg als
Hauptweg und den nativen Start als Alternative, falls Container Manager nach einem
DSM-Update einmal verschwindet.

Der Engpass ist damit nicht die Containerfähigkeit, sondern **der Arbeitsspeicher**.

---

## Warum trotzdem ein einzelnes Binary

**Backend in Go statt Python.** Das revidiert E2 — und der Grund ist jetzt nicht
mehr das fehlende Docker, sondern das 1 GB RAM, das sich schon eine ganze Reihe
laufender Pakete teilt (Container Manager, Antivirus Essential, Active Insight,
Advanced Media Extensions, Download Station, File Station …). Der Docker-Daemon
belegt davon allein 100–200 MB.

Die inhaltliche Begründung bleibt dieselbe: Die Begründung für Python waren Tokenizer, Embedding-Bibliotheken und
Retrieval-Werkzeuge. Genau die brauchen wir hier nicht — Embeddings kommen von
OpenRouter (E3), die Suche macht SQLite, und Tokenzahlen liefert OpenRouter im
`usage`-Objekt zurück. Übrig bleibt HTTP, SSE, JSON und SQLite. Das kann Go, und
es kann es in einer Form, die auf dieser Hardware alles andere schlägt:

|  | Go | Python |
|---|---|---|
| Image-Größe | ~15 MB (`FROM scratch`) | ~150 MB |
| RAM im Betrieb | ~25–50 MB | ~100–150 MB |
| ohne Docker startbar | **ja, eine Datei** | Python-Paket, venv, `pip install` über eine schwache CPU |
| Frontend | ins Binary eingebettet (`embed.FS`) | separat auszuliefern |

Bei 1 GB, das sich DSM mit einem halben Dutzend Paketen teilt, ist der Unterschied
nicht kosmetisch — er entscheidet, ob die NAS entspannt läuft oder swappt.

Das React-Frontend bleibt wie geplant — es wird in GitHub Actions gebaut und ins
Binary eingebettet. **Auf der NAS läuft nie ein Build**, nur das fertige Programm.

---

## Weg A: Container Manager (Hauptweg)

GitHub Actions baut bei jedem Release ein arm64-Image und legt es auf GHCR ab.

1. **Ordner anlegen:** In der File Station `/volume1/docker/plot/` erstellen.
2. **Projekt anlegen:** Container Manager → Projekt → Erstellen, als Quelle die
   `docker-compose.yml` aus dem Repo einfügen (oder hochladen). Sie mountet
   `/volume1/docker/plot` als Datenordner und veröffentlicht Port 8080.
3. **Starten.** Container Manager zieht das Image und startet es; Autostart nach
   einem Neustart der NAS ist eingebaut.
4. **Aufrufen:** `http://<tailscale-name>:8080`, den OpenRouter-Key trägst du
   **in der Weboberfläche** ein — du sollst nie einen Texteditor auf der NAS
   brauchen.

Updates: im Projekt auf „Erstellen"/Pull klicken, das Image wird ersetzt.

## Weg B: nativ, ohne Docker (Alternative)

Sinnvoll, wenn der Arbeitsspeicher knapp wird oder Container Manager nach einem
DSM-Update einmal nicht mehr da ist. Spart die 100–200 MB des Docker-Daemons.

1. `plot-linux-arm64` aus dem Release herunterladen, per File Station nach
   `/volume1/plot/` ziehen.
2. Systemsteuerung → Aufgabenplaner → Erstellen → Ausgelöste Aufgabe →
   Benutzerdefiniertes Skript. Ereignis „Hochfahren", Benutzer `root`, im
   Skriptfeld eine Zeile:
   ```
   /volume1/plot/plot-linux-arm64 --data /volume1/plot &
   ```
   „Jetzt ausführen" startet es sofort, ohne Neustart.

Kein SSH, kein Terminal, kein Paketmanager. Dieselbe SQLite-Datei funktioniert für
beide Wege — du kannst jederzeit wechseln, ohne etwas zu verlieren.

## Zugriff (Tailscale ist schon da)

Der Verkehr im Tailnet läuft über WireGuard und ist bereits verschlüsselt — für den
Zugriff von unterwegs brauchst du also **kein** Zertifikat und keinen Reverse Proxy.
HTTP auf Port 8080 innerhalb des Tailnets genügt.

Der Login in der App bleibt trotzdem drin: falls je ein weiteres Gerät oder eine
weitere Person ins Tailnet kommt, oder ein Gerät verloren geht. Ein Passwort
(argon2) und ein Session-Cookie, mehr nicht.

Der OpenRouter-Key liegt serverseitig in `/volume1/plot/` und geht nie ans Frontend.

---

## Was die Hardware bedeutet

**Was problemlos läuft:** Die Arbeit dieses Programms ist Warten auf OpenRouter,
dazu ein bisschen JSON und ein paar SQLite-Abfragen. Dafür sind vier A55-Kerne
reichlich. Die Antwortzeit bestimmt das Modell, nicht die NAS.

**Grenzen, ehrlich:**

- **Ein Nutzer, eine Session gleichzeitig.** Für dich ausgelegt, nicht für eine
  Gruppe. Mehrere Geräte gleichzeitig im selben Chat wären ein anderes Projekt.
- **Keine lokalen Embeddings, kein lokales Modell.** War ohnehin entschieden (E3),
  ist jetzt aber keine Option mehr, sondern ausgeschlossen.
- **Vektorsuche startet als Stichwortsuche.** SQLite FTS5 ist eingebaut und kostet
  nichts. Die Vektor-Erweiterung `sqlite-vec` müsste als arm64-Modul dazu —
  machbar, aber ich prüfe das erst in M3, wenn sich zeigt, dass BM25 nicht reicht.
  Für Eigennamen, Zitate und Daten — das, was im Rollenspiel am häufigsten gesucht
  wird — ist Stichwortsuche ohnehin die bessere Hälfte.
- **Der Arbeitsspeicher ist der eigentliche Engpass.** Schau vor der Installation
  einmal in Systemsteuerung → Ressourcen-Monitor, wie viel wirklich frei ist.
  Bleiben unter ~150 MB übrig, lohnt es sich, nicht benötigte Pakete zu stoppen
  (Antivirus Essential und Download Station laufen bei dir dauerhaft mit) — oder
  gleich Weg B ohne Docker zu nehmen.
- **Wenn ein Backup oder eine Medienindizierung parallel läuft**, wird es kurz zäh.
  Nichts geht kaputt, es dauert nur.
- **Kein Prompt-Caching**, aber das lag ohnehin am Anbieter (siehe modelle.md), nicht
  an der NAS.

**Wenn es dir zu eng wird:** Ein Raspberry Pi 5 oder ein gebrauchter Mini-PC
(~60–150 €) nimmt dasselbe Binary ohne Änderung — Go baut für beide Architekturen
aus demselben Quelltext. Die SQLite-Datei kopierst du rüber, fertig. **Jetzt musst
du nichts kaufen**; das ist der Ausweg, falls die DS124 irgendwann bremst, nicht
eine Voraussetzung.

---

## Backup

Hyper Backup auf `/volume1/plot/`. Die App schreibt vor jedem Backup-Zeitfenster
einmal täglich per `VACUUM INTO` eine konsistente Kopie neben die Live-Datenbank,
damit auch bei laufendem WAL nichts halbfertig gesichert wird. Dazu der
App-eigene Story-Export als `.zip`.
