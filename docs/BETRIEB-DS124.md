# Betrieb auf der DS124

## Der Befund

| | |
|---|---|
| CPU | Realtek RTD1619B, 4 × ARM Cortex-A55, 1,7 GHz (arm64) |
| RAM | 1 GB DDR4, **nicht erweiterbar** |
| DSM | 7.3.2 |
| Container Manager | **nicht verfügbar** |

Container Manager setzt einen Intel- oder AMD-Prozessor voraus. Auf Realtek-basierten
Modellen — DS124, DS223, DS223j und der Rest der Value-Linie — erscheint das Paket
gar nicht erst im Package Center, und es gibt keinen offiziellen Weg drumherum.

Das Docker-Konzept aus der ersten Planung fällt damit weg. Es geht trotzdem, aber
anders — und die Einschränkung erzwingt zwei Änderungen, die das Ergebnis
unterm Strich sogar robuster machen.

---

## Die Lösung: ein einzelnes Binary

**Backend in Go statt Python.** Das revidiert E2, und zwar aus einem konkreten
Grund: Die Begründung für Python waren Tokenizer, Embedding-Bibliotheken und
Retrieval-Werkzeuge. Genau die brauchen wir hier nicht — Embeddings kommen von
OpenRouter (E3), die Suche macht SQLite, und Tokenzahlen liefert OpenRouter im
`usage`-Objekt zurück. Übrig bleibt HTTP, SSE, JSON und SQLite. Das kann Go, und
es kann es in einer Form, die auf dieser Hardware alles andere schlägt:

|  | Go | Python |
|---|---|---|
| Installation | **eine Datei hochladen** | Python-Paket, venv, `pip install` über eine schwache CPU |
| SSH nötig | **nein** | praktisch ja |
| RAM im Betrieb | ~25–50 MB | ~100–150 MB |
| DSM-Update überlebt | ja, es ist nur eine Datei | Paketwechsel kann das venv zerlegen |
| Frontend | ins Binary eingebettet (`embed.FS`) | separat auszuliefern |

Bei 1 GB RAM, von dem DSM selbst den Großteil belegt, ist der Unterschied nicht
kosmetisch. Und für dich, der nicht im Code arbeitet, ist „eine Datei ersetzen und
neu starten" das ganze Update-Verfahren.

Das React-Frontend bleibt wie geplant — es wird in GitHub Actions gebaut und ins
Binary eingebettet. **Auf der NAS läuft nie ein Build**, nur das fertige Programm.

---

## Installation ohne SSH

1. **Binary holen:** GitHub Actions baut bei jedem Release ein `plot-linux-arm64`.
   Herunterladen, in der File Station nach `/volume1/plot/` ziehen.
2. **Konfiguration:** Beim ersten Start legt das Programm `/volume1/plot/config.json`
   an. Den OpenRouter-Key trägst du danach **in der Weboberfläche** ein, nicht in
   der Datei — du sollst nie einen Texteditor auf der NAS brauchen.
3. **Autostart:** Systemsteuerung → Aufgabenplaner → Erstellen → Ausgelöste Aufgabe
   → Benutzerdefiniertes Skript. Ereignis „Hochfahren", Benutzer `root`, im
   Skriptfeld eine Zeile:
   ```
   /volume1/plot/plot-linux-arm64 --data /volume1/plot &
   ```
   „Jetzt ausführen" startet es sofort, ohne Neustart.
4. **Aufrufen:** `http://<tailscale-name>:8080` von jedem Gerät im Tailnet.

Kein SSH, kein Terminal, kein Paketmanager. Updates: neue Datei hochladen, Aufgabe
einmal stoppen und starten.

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
- **Wenn Synology Photos oder ein Backup parallel indexiert**, wird es kurz zäh.
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
