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
| Image-Größe | **17 MB** (gemessen, `FROM scratch`) | ~150 MB |
| RAM im Betrieb | **3,4 MB im Leerlauf** (gemessen), unter Last deutlich unter 50 MB | ~100–150 MB |
| ohne Docker startbar | **ja, eine Datei** | Python-Paket, venv, `pip install` über eine schwache CPU |
| Frontend | ins Binary eingebettet (`embed.FS`) | separat auszuliefern |

Bei 1 GB, das sich DSM mit einem halben Dutzend Paketen teilt, ist der Unterschied
nicht kosmetisch — er entscheidet, ob die NAS entspannt läuft oder swappt.

Das React-Frontend bleibt wie geplant — es wird in GitHub Actions gebaut und ins
Binary eingebettet. **Auf der NAS läuft nie ein Build**, nur das fertige Programm.

---

## Weg A: Container Manager

**Variante 1 — Abbild als Datei einspielen.** Braucht kein Registry-Konto und
keine Freigabe von Paketsichtbarkeiten.

1. Container Manager → **Abbild** → Hinzufügen → **Von Datei hinzufügen**, die
   Datei `plot-arm64-image.tar` auswählen. Danach steht `plot:0.1.0` in der Liste.
2. File Station → Ordner `/volume1/docker/plot` anlegen (er nimmt die Datenbank auf).
3. Das Abbild markieren → **Starten**. Im Assistenten:
   - Container-Name: `plot`
   - Automatischen Neustart aktivieren: Häkchen
   - Port-Einstellungen: lokaler Port **8080** → Container-Port **8080**
   - Ordner: `/volume1/docker/plot` einhängen als **`/data`**
4. Fertig, läuft.

Wer lieber ein Projekt anlegt, nimmt `docker-compose.lokal.yml` aus dem Repo —
sie verweist auf das importierte Abbild und holt nichts aus einer Registry.

**Variante 2 — aus der Registry ziehen.** Bequemer bei Updates, verlangt aber
einmalige Vorarbeit: Das Abbild entsteht im Arbeitsablauf „Veröffentlichen"
(GitHub → Actions → Run workflow), und ein neu angelegtes GHCR-Paket ist
**privat**, auch bei öffentlichem Repository. Entweder du stellst es einmalig um
(GitHub → Profil → Packages → `plot` → Package settings → Change visibility →
Public), oder du hinterlegst im Container Manager unter Registrierung →
Einstellungen ein Konto für `ghcr.io` mit einem Zugriffstoken. Danach: Projekt
anlegen mit der `docker-compose.yml` aus dem Repo.

### Aktualisieren

Jeder Merge nach main veröffentlicht ein neues `:latest`. Auf der NAS holt es
[`scripts/update-synology.sh`](../scripts/update-synology.sh) ab:

1. Skript per File Station nach `/volume1/docker/plot/` legen.
2. Systemsteuerung → Aufgabenplaner → Erstellen → **Geplante Aufgabe** →
   Benutzerdefiniertes Skript, Benutzer `root`, im Skriptfeld:
   ```sh
   sh /volume1/docker/plot/update-synology.sh
   ```
3. Zeitplan nach Geschmack (etwa nachts) — oder ohne Zeitplan anlegen und die
   Aufgabe bei Bedarf über „Ausführen" auslösen.

Das Skript holt das Abbild, startet den Container nur dann neu, wenn sich
tatsächlich etwas geändert hat, räumt abgelöste Abbilder weg und schreibt nach
`/volume1/docker/plot/update.log`. Findet es die Compose-Datei des Projekts,
benutzt es sie; sonst startet es den Container direkt neu. Der Datenordner
bleibt unangetastet, die Datenbank überlebt jedes Update.

## Weg B: als Binary, ohne Docker

Sinnvoll, wenn der Arbeitsspeicher knapp wird: spart die 100–200 MB des
Docker-Daemons. Dieselbe Datenbank funktioniert für beide Wege — du kannst
jederzeit wechseln, ohne etwas zu verlieren.

**1. Ordner anlegen.** File Station → freigegebenen Ordner `plot` erstellen.
Sein Pfad ist dann `/volume1/plot`.

**2. Binary hineinlegen.** `plot-linux-arm64` per Drag & Drop in den Ordner ziehen.

**3. Autostart einrichten.** Systemsteuerung → Aufgabenplaner → Erstellen →
Ausgelöste Aufgabe → Benutzerdefiniertes Skript:

- Aufgabenname: `Plot`
- Benutzer: `root`
- Ereignis: `Hochfahren`
- Aktiviert: Häkchen setzen
- Reiter „Aufgabeneinstellungen" → Benutzerdefiniertes Skript:

```sh
chmod +x /volume1/plot/plot-linux-arm64
/volume1/plot/plot-linux-arm64 --data /volume1/plot &
```

Die erste Zeile ist nicht optional: über File Station hochgeladene Dateien sind
nicht ausführbar, und ohne sie startet nichts.

**4. Starten.** Aufgabe in der Liste markieren → „Ausführen". Kein Neustart nötig.

**Stoppen** (etwa vor einem Update): eine zweite Aufgabe nach demselben Muster,
ohne Ereignis, mit `pkill -f plot-linux-arm64` als Skript. „Beenden" im
Aufgabenplaner stoppt den Hintergrundprozess nicht zuverlässig.

**Aktualisieren:** stoppen, neue Datei hochladen (überschreiben), Startaufgabe
erneut ausführen. Die Datenbank bleibt unangetastet.

Achtung beim Wechsel zwischen den Wegen: Der Container legt die Datenbank unter
`/volume1/docker/plot` an, die Aufgabenplaner-Variante oben unter `/volume1/plot`.
Wer wechselt, gibt entweder denselben Ordner an oder kopiert `plot.sqlite` hinüber.

## Zugriff (Tailscale ist schon da)

Der Verkehr im Tailnet läuft über WireGuard und ist bereits verschlüsselt — für den
Zugriff von unterwegs brauchst du also **kein** Zertifikat und keinen Reverse Proxy.
HTTP auf Port 8080 innerhalb des Tailnets genügt.

Aufgerufen wird `http://<gerätename>:8080` (bei aktivem MagicDNS) oder
`http://100.x.y.z:8080` mit der Tailscale-Adresse der NAS; beides steht in der
Tailscale-App unter dem Gerät. Zwei mögliche Stolpersteine: Ist Port 8080 auf der
NAS schon belegt, hängst du `--addr :8099` an den Startbefehl. Und wenn die
DSM-Firewall aktiv ist, muss der Port für das Tailscale-Netz freigegeben sein.

Der Login in der App bleibt trotzdem drin: falls je ein weiteres Gerät oder eine
weitere Person ins Tailnet kommt, oder ein Gerät verloren geht. Ein Passwort
(argon2) und ein Session-Cookie, mehr nicht.

Der OpenRouter-Key liegt serverseitig in `/volume1/plot/` und geht nie ans Frontend.

---

## Was die Hardware bedeutet

**Was problemlos läuft:** Die Arbeit dieses Programms ist Warten auf OpenRouter,
dazu ein bisschen JSON und ein paar SQLite-Abfragen. Dafür sind vier A55-Kerne
reichlich. Die Antwortzeit bestimmt das Modell, nicht die NAS.

Gemessen am fertigen Container: **17 MB Image, 3,4 MB Arbeitsspeicher im
Leerlauf.** Damit ist die Sorge um den knappen Speicher weitgehend erledigt —
der Docker-Daemon selbst braucht mehr als die Anwendung darin.

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
- **Der Arbeitsspeicher bleibt der Punkt, den man im Auge behält** — allerdings
  wegen des Docker-Daemons, nicht wegen der Anwendung. Schau einmal in
  Systemsteuerung → Ressourcen-Monitor. Wird es eng, spart Weg B den Daemon
  komplett ein und kostet dich nur die 3–4 MB des Programms selbst.
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
