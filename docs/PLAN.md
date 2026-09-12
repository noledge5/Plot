# Plot — Architektur- und Umsetzungsplan

Lokale Rollenspiel-Engine mit eigenem System-Prompt, Charakterkarten, Beziehungs-
und Gedächtnismechanik. Läuft als ein einzelnes Binary auf einer Synology DS124,
bedienbar von jedem Gerät im Tailnet (Browser/PWA).

Getroffene Entscheidungen: [ENTSCHEIDUNGEN.md](ENTSCHEIDUNGEN.md) ·
Noch offen: [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) ·
Prompt-Startvorlage: [system-prompt-vorlage.md](system-prompt-vorlage.md)

---

## 1. Leitprinzipien

1. **Der System-Prompt gehört dir.** Die Engine injiziert nie versteckten Text.
   Alles steht in deinem Template oder in einem Platzhalter, den du gesetzt hast.
   Ein Inspektor zeigt den exakten Request.
2. **Zustand ist Daten, nicht Prosa.** Beziehungen, Fakten, Grenzen und Antriebe
   liegen strukturiert in SQLite. Das Modell erzählt; es verwaltet nichts.
3. **Das Modell schlägt vor, die Engine entscheidet.** Zustandsänderungen kommen
   als JSON zurück, werden gedeckelt, geloggt und bleiben editierbar.
4. **Gefälligkeit wird gemessen, nicht erhofft.** Siehe §9.
5. **Nichts geht verloren.** Jeder Turn ist ein Knoten im Baum; Regenerieren
   erzeugt einen Zweig.
6. **Ein Container, keine Fremddienste.** Kein Postgres, kein Redis, keine
   separate Vektordatenbank.

---

## 2. Systemüberblick

```
   Handy / Laptop / Tablet
            │  HTTPS (Tailscale oder DSM Reverse Proxy)
            ▼
   ┌─────────────────────────────────────────────┐
   │  Go-Binary auf der DS124 (Container o. nativ)│
   │                                             │
   │   React-PWA  ──►  API (eingebettet)          │
   │                     │                       │
   │                     ├─ Prompt-Builder ──────┼──► OpenRouter
   │                     ├─ Autonomie-Schicht    │     /chat/completions (Erzählung)
   │                     ├─ State-Extractor      │     /chat/completions (Utility)
   │                     ├─ Memory-Indexer       │     /embeddings
   │                     ▼                       │
   │                  SQLite (WAL) + FTS5        │
   └─────────────────────────────────────────────┘
                         │
            /volume1/plot ──► Hyper Backup
```

Ein Prozess, eine Datei, ein Ordner. Kein Zustand außerhalb von `/volume1/plot`.

Weil du nicht selbst im Code arbeitest (E2), gilt zusätzlich: **jede Einstellung
ist in der Oberfläche erreichbar** — Modelle, Provider-Filter, Budgets, Prompts,
Schwellenwerte der Autonomie-Schicht. Die einzige Datei, die du je anfassen musst,
trägst du in der Weboberfläche ein: den OpenRouter-Key beim ersten Start.

---

## 3. Stack

- **Backend:** Go, ein einzelnes statisches Binary für arm64 — ausgeliefert als
  schlankes Container-Image und als nackte Datei. Keine Laufzeitumgebung, keine
  Abhängigkeiten auf dem Zielgerät; entscheidend ist das 1 GB RAM der DS124.
  Begründung und Installationswege: [BETRIEB-DS124.md](BETRIEB-DS124.md).
- **Frontend:** React + Vite + TypeScript + Tailwind, in GitHub Actions gebaut und
  per `embed.FS` ins Binary eingebettet. PWA-Manifest für „Zum Homescreen".
  Auf der NAS läuft nie ein Build.
- **DB:** SQLite im WAL-Modus, FTS5 für Stichwortsuche. Vektoren (`sqlite-vec`)
  erst ab M3 und nur, wenn BM25 nachweislich nicht reicht.

---

## 4. Datenmodell

**Event-Sourcing auf einem Baum.** Beziehungswerte und gültige Fakten werden nicht
gespeichert, sondern aus dem Pfad von der Wurzel zum aktuellen Knoten gefaltet.
Dadurch sind Speicherslots und Verzweigungen automatisch korrekt: ein Zweig kann
keinen Zustand aus einem anderen sehen. Snapshots sind nur Cache.

```sql
story            (id, title, created_at, settings_json)
prompt_template  (id, story_id, kind, name, body, version, created_at)
                 -- kind: narrator | extractor | summarizer | timeskip | ...

persona          (id, story_id, name, sheet_json)
character        (id, story_id, name, sheet_json, avatar_path)
scene            (id, story_id, location, ingame_time, present_character_ids)

node             (id, story_id, parent_id, kind, role, content,
                  scene_id, model, usage_json, flags_json, created_at)
                 -- kind: turn | narration | event | timeskip | note
                 -- flags_json: Befunde der Autonomie-Schicht (§9)

save_slot        (id, story_id, node_id, name, kind, created_at, preview)

memory_fact      (id, story_id, subject_id, kind, text, importance, confidence,
                  status, ingame_time, valid_from_node, valid_to_node,
                  source_node_id, pinned)
                 -- status: proposed | canon | retired
memory_vec       (fact_id, embedding)              -- sqlite-vec
memory_fts       (fact_id, text)                   -- FTS5
memory_summary   (id, story_id, level, from_node, to_node, text)
lore_entry       (id, story_id, keys_json, text, priority, always_on)

knowledge        (character_id, fact_id, since_node, how)
                 -- how: witnessed | told | inferred | assumed(falsch!)

rel_delta        (id, node_id, from_char, to_char, axis, delta, reason, quote)
rel_snapshot     (node_id, from_char, to_char, axis, value)

concession       (id, node_id, character_id, scale, topic, covered_by_state)
                 -- Protokoll der Zugeständnisse, Grundlage des Widerstandsbudgets

run_log          (id, node_id, purpose, request_json, response_meta_json,
                  cost_usd, latency_ms)
```

`run_log` ist nicht optional: ohne den exakt abgeschickten Payload lässt sich nie
klären, ob eine schlechte Antwort am Modell oder am Prompt lag.

### Charakterblatt (`character.sheet_json`)

Die Felder, die die Autonomie-Schicht braucht, sind **eigene Felder, keine Prosa**:

```jsonc
{
  "kern":        "…zwei, drei Sätze, wer sie ist…",
  "sprechweise": "…Satzbau, Wortwahl, Eigenheiten…",
  "drives":      [ { "ziel": "…", "druck": 0-100, "sichtbar": true } ],
  "hard_limits": [ "…was sie niemals tut, egal bei welchem Wert…" ],
  "soft_limits": [ { "was": "…", "erst_ab": { "trust": 60 } } ],
  "deal_breakers": [ "…was Werte drastisch senkt…" ],
  "volatility":  { "trust": 0.4, "warmth": 1.2, "…": 1.0 },
  "secrets":     [ { "text": "…", "preisgabe_ab": { "trust": 75 } } ]
}
```

`volatility` unter 1 heißt: diese Achse bewegt sich bei dieser Figur langsamer.
Eine misstrauische Figur baut Vertrauen langsam auf und verliert es schnell — das
allein erzeugt schon spürbar unterschiedliche Persönlichkeiten.

---

## 5. Der Prompt-Builder

Du schreibst den System-Prompt selbst (Startvorlage liegt bei). Platzhalter:

```
{{persona}}  {{characters}}  {{relationships}}  {{world}}  {{lore}}
{{memories}} {{summary}}     {{scene}}          {{time}}   {{style}}
{{drives}}   {{limits}}      {{directives}}     {{author_note}}
```

`{{directives}}` ist der Kanal, über den die Autonomie-Schicht spricht — dort
landen die harten Anweisungen aus §9. Du entscheidest, wo im Prompt er steht.

Regeln des Builders:

- **Cache-freundliche Reihenfolge:** stabile Blöcke zuerst (System-Prompt,
  Charaktere, Welt, Lore), volatile zuletzt (Memories, Direktiven, Verlauf). Bei
  Anthropic/Qwen setzt der Builder automatisch `cache_control`-Breakpoints an die
  Grenze; bei OpenAI/DeepSeek/Gemini greift implizites Caching. Der teure Präfix
  wird so pro Turn nur einmal voll bezahlt.
- **Budget-Manager:** jeder Block hat Kontingent und Priorität. Wird es eng, wird
  von unten gekürzt — älteste wörtliche Turns wandern zuerst in die
  Zusammenfassung. Was gekürzt wurde, steht im Inspektor.
- **Zahlen werden übersetzt.** `trust=22` geht nie als Zahl ins Prompt, sondern als
  Verhalten: *„Mira misstraut dir. Sie weicht persönlichen Fragen aus und prüft,
  ob deine Aussagen zu dem passen, was sie schon weiß."* Modelle befolgen
  Verhaltensbeschreibungen deutlich zuverlässiger als Skalen.
- **Wissensfilter:** `{{memories}}` und `{{characters}}` werden pro anwesender
  Figur gefiltert (§8.3).

### 5.1 Prompt-Diät (Folge von E12)

Kleine Modelle verlieren Anweisungen in langen Prompts — mehr Kontext heißt bei
ihnen schlechtere Regelbefolgung, nicht bessere. Drei Regeln, die bei einem
Spitzenmodell überflüssig wären:

- **Harte Obergrenze weit unter dem Kontextfenster.** Dolphin kann 128k; der
  Builder zielt trotzdem auf 6–8k. Der Platz ist da, die Aufmerksamkeit nicht.
- **Regeln rotieren statt stapeln.** Nicht alle Verhaltensregeln in jedem Turn:
  die Engine wählt die zwei, drei aus, die zur Lage passen — „Zustimmung ist
  teuer" etwa nur, wenn tatsächlich eine Bitte im Raum steht.
- **Direktiven ans Ende.** Bei kleinen Modellen schlägt Nähe zum Ende alles
  andere. `{{directives}}` steht deshalb hinter dem Verlauf, und die ein, zwei
  wichtigsten Regeln werden dort kurz wiederholt.

---

## 6. Gedächtnis

| Ebene | Inhalt | Erzeugung |
|---|---|---|
| L0 | letzte N Turns wörtlich | — |
| L1 | Szenen-/Kapitel-/Akt-Zusammenfassung | billiges Modell, rollierend |
| L2 | Fakten-Wiki: atomare, editierbare Aussagen mit Herkunft und Zeitpunkt | Extraktion + deine Korrektur |
| L3 | Lorebook: statisches Weltwissen, per Stichwort | von dir geschrieben |

**Retrieval ist hybrid.** Reine Vektorsuche versagt bei genau den Fragen, die im
Rollenspiel zählen — Eigennamen, Daten, „was hat er damals gesagt":

```
score = w1·cosine + w2·bm25 + w3·importance + w4·recency + w5·betrifft_anwesende
immer dabei: pinned facts, Fakten über Anwesende, offene Konflikte, Geheimnisse
             der sprechenden Figur
```

**Extraktion läuft asynchron** nach dem Turn und blockiert die Antwort nicht. Ein
Utility-Call mit striktem JSON-Schema liefert Kandidaten, dann:

1. **Dedup:** Ähnlichkeit über Schwelle → bestehenden Fakt aktualisieren.
2. **Widerspruch:** neuer Fakt widerspricht altem → alter bekommt `valid_to_node`
   und `retired`, statt gelöscht zu werden. So bleibt „sie *war* Ärztin, bis sie
   gekündigt hat" korrekt darstellbar.
3. **Review:** neue Fakten landen je nach Einstellung als `proposed` in einer
   Warteschlange oder direkt als `canon` **[Q6]**.

---

## 7. Beziehungen

Gerichtete Achsen, weil Sympathie nicht symmetrisch ist (A→B ≠ B→A):

`trust, warmth, attraction, respect, familiarity, tension, resentment, obligation, fear`

Werte −100…+100. Nach jedem Turn ein billiger Utility-Call mit
`response_format: json_schema`:

```json
{"deltas":[{"from":"mira","to":"player","axis":"trust","delta":-6,
            "reason":"Er hat ihre Frage nach dem Brief ausweichend beantwortet.",
            "quote":"…"}]}
```

Die Engine **clampt** (Grundwert ±5 pro Turn, multipliziert mit `volatility`),
loggt Begründung samt Zitat, schreibt einen `rel_delta`. Jeder Delta ist im UI
korrigierbar; der Zustand wird dann neu gefaltet. Ohne Deckelung springen Modelle
nach einem einzigen netten Satz von Misstrauen auf Zuneigung — der häufigste
Realismus-Killer überhaupt.

**Drift:** ohne Kontakt bewegen sich `warmth` und `tension` über In-Game-Zeit
langsam Richtung Baseline.

---

## 8. Welt und Wissen

### 8.1 Gruppenszenen (E8)

Im Spielleiter-Modus erzählt ein Aufruf die ganze Szene mit allen Anwesenden.
Damit das nicht in Einheitsbrei kippt:

- **Beziehungen der NPCs untereinander** stehen mit im Prompt — sie sind der Motor
  jeder Gruppenszene. Wer wen nicht ausstehen kann, ist interessanter als jede
  Beschreibung.
- **Wissensblöcke pro Figur** statt eines gemeinsamen Kontexts: „Was Mira weiß:
  … / Was Jonas weiß: …". Der Erzähler ist allwissend, die Figuren sind es nicht.
- **Sprecherregie:** eine Zeile im Prompt, wer gerade Anlass hat zu reden — aus
  Antriebsdruck und Spannungswerten abgeleitet, nicht zufällig.

### 8.2 In-Game-Zeit (E9)

Jede Szene trägt einen Zeitstempel, jeder Fakt den Zeitpunkt seiner Entstehung.
Daraus folgen kostenlos: „vor drei Wochen", Termine, Fristen, Beziehungsdrift.

**Zeitsprung-Zusammenfassung** statt Off-Screen-Simulation (E11): springst du
explizit in der Zeit, fragt ein einziger Utility-Call, was in der Zwischenzeit bei
den Figuren passiert ist — entlang ihrer Antriebe, nicht beliebig. Das Ergebnis
zeigt die Engine dir zur Freigabe, bevor es Kanon wird. Kostet einen Call pro
Sprung statt permanenter Ticks und liefert den größten Teil des Effekts.

### 8.3 Wissensmodell

Jeder Fakt hat eine `known_by`-Menge mit *wie*: `witnessed`, `told`, `inferred`,
`assumed`. Beim Prompt-Bau wird pro Figur gefiltert. Was Mira nicht miterlebt hat
und ihr niemand erzählt hat, steht nicht in ihrem Block.

Das ist der größte Realismus-Hebel im ganzen System. Daraus entstehen von allein
die Momente, die echt wirken: Missverständnisse, Nachfragen, jemand erfährt etwas
zu spät. `assumed` erlaubt sogar bewusst falsche Annahmen einer Figur.

### 8.4 Kontinuitätsprüfung

Optionaler Pass nach der Antwort: prüft gegen `canon`-Fakten und markiert
Widersprüche als Warnung. **Nicht** automatisch umschreiben — sonst korrigierst du
hinterher die Korrektur.

---

## 9. Die Autonomie-Schicht (Kernfeature)

Das Problem in einem Satz: **Modelle sind darauf trainiert, dir zu gefallen.** Eine
Prompt-Regel dagegen wirkt zehn Turns lang und verliert dann gegen das Training —
zuverlässig. Deshalb vier Ebenen statt einer, von weich nach hart.

### 9.1 Textregeln (Basis, ab M2)

Im Stilleitfaden, in deinen Worten. Die Startvorlage enthält einen Vorschlag.
Wirkt, reicht aber nicht.

### 9.2 Grenzen als harte Daten (ab M2)

`hard_limits`, `soft_limits` mit Schwellen, `deal_breakers` und `secrets` mit
Preisgabe-Schwelle stehen als eigene Felder im Charakterblatt und werden wörtlich
in `{{limits}}` gerendert — nicht als Fließtext, den das Modell überliest, sondern
als Liste. Ein `hard_limit` gilt bei jedem Beziehungswert, ausnahmslos.

### 9.3 Gefälligkeits-Detektor (ab M2)

Ein billiger Utility-Call prüft jede Antwort auf sieben konkrete Muster:

| # | Muster |
|---|---|
| 1 | Zustimmung ohne Gegenleistung oder Bedenkzeit |
| 2 | Spiegeln der Spieleremotion statt eigener Reaktion |
| 3 | unaufgefordertes Lob, Bestätigung, Bewunderung |
| 4 | heikle Frage wird bereitwillig und vollständig beantwortet |
| 5 | Konflikt wird weichgespült statt ausgetragen |
| 6 | Figur ergreift keine eigene Initiative — kein Antrieb sichtbar |
| 7 | die Antwort lässt deine Figur handeln oder sprechen (E5) |

Befunde landen in `node.flags_json` und werden im UI am Turn angezeigt, mit einem
Knopf „nochmal, härter" — der regeneriert mit einer verschärften Direktive in
`{{directives}}`. Standardmäßig markieren, nicht automatisch verwerfen; auf Wunsch
umstellbar auf „bei Muster 7 immer automatisch neu".

### 9.4 Widerstandsbudget und Konzessionsprüfung (ab M4)

Die härteste Ebene, weil sie zählt statt zu bitten.

**Konzessionsskala.** Der Extractor stuft jedes Nachgeben ein:

| Stufe | Bedeutung |
|---|---|
| 0 | kein Nachgeben |
| 1 | kleine Gefälligkeit |
| 2 | spürbares Zugeständnis |
| 3 | Grenzüberschreitung, Geheimnispreisgabe, Kernposition aufgegeben |

**Deckung durch den Zustand.** Stufe 2 und 3 brauchen einen Beziehungswert, der
das trägt (aus `soft_limits` / `secrets.preisgabe_ab`). Fehlt die Deckung, wird der
Turn als *ungedeckte Konzession* markiert — samt Angabe, welcher Wert fehlt
(„gibt das Geheimnis preis, braucht trust 75, hat 24").

**Budget.** Die Engine zählt Konzessionen ≥ 2 pro Szene und pro Figur. Ab der
zweiten schreibt sie eine harte Direktive in den nächsten Prompt:

> Mira hat in dieser Szene bereits zweimal nachgegeben. Sie gibt jetzt nicht nach.
> Sie hat einen eigenen Grund, warum nicht, und sie benennt ihn.

**Antriebsdruck.** Jeder `drive` hat einen Druckwert, der steigt, solange das Ziel
nicht vorankommt. Über der Schwelle erzeugt die Engine ebenfalls eine Direktive:
*„Mira braucht bis Freitag Geld. Sie lenkt das Gespräch aktiv darauf, auch wenn es
gerade unpassend ist."* Das ist die Mechanik, die aus reagierenden NPCs handelnde
macht — ohne sie bleiben Figuren höflich abwartend, egal was im Prompt steht.

Alle Schwellen (Budget, Deckungswerte, Druckgrenze, Clamp) sind in der Oberfläche
einstellbar. Ich setze Startwerte, du drehst daran, bis der Ton stimmt.

---

## 10. Speicherslots und Verzweigung

- **Regenerieren** = Geschwisterknoten; die alte Fassung bleibt.
- **Text ändern** korrigiert einen Zug an Ort und Stelle - für Tippfehler und
  kleine Eingriffe, ohne einen Zweig zu erzeugen.
- **Neu erzählen lassen** an einem alten Zug = neuer Zweig ab dort. Fakten mit
  `valid_from_node` außerhalb des neuen Pfads sind dort automatisch ungültig — das
  Gedächtnis vergisst korrekt, was in der verworfenen Zeitlinie geschah.
- **Speicherslot** = benannter Zeiger auf einen Knoten. Laden heißt: Zeiger setzen.
- **Autosave** alle N Turns, mit Rotation.
- **Export** einer Story als `.zip` (SQLite-Auszug + JSON + Bilder).

Die UI zeigt standardmäßig nur die Slot-Liste; die Baumansicht ist ein Schalter.

---

## 11. Modelle (kleine, günstige Klasse — E12)

Details, Preise und die Kandidatenliste: [modelle.md](modelle.md).
Die drei Punkte, die die Architektur betreffen:

**Drei Rollen statt zwei.** Erzähler, **Analyst** und Reserve-Erzähler. Der
Analyst ist eine eigene Rolle, weil Dolphin (`Venice: Uncensored`) laut
OpenRouter-Katalog kein `structured_outputs` unterstützt — nur den einfachen
JSON-Modus. Die ganze Zustandsmechanik aus §7 und §9 hängt aber an
schema-validiertem JSON. Also läuft sie auf einem zweiten, sehr billigen Modell,
und der Erzähler macht nur, was er gut kann.

Wichtig dabei: der Analyst liest den erzeugten Text mit und muss ihn deshalb
verarbeiten *dürfen*. Ein moderiertes Utility-Modell verweigert sonst genau dann,
wenn es gebraucht wird.

**Kosten sind kein Engpass mehr, Regelbefolgung ist es.** Ein Turn kostet rund
0,19 Cent, 1.000 Turns keine zwei Euro (Rechnung in modelle.md §5). Zwei Folgen:
Analyst-Calls dürfen großzügig laufen, und der automatische Neuversuch bei
Detektor-Befund (§9.3) wird praktisch gratis — bei einem teuren Modell hätte ich
davon abgeraten. Der Budget-Manager aus §5 bleibt, dient aber ab jetzt der
Qualität, nicht dem Sparen.

**Kein Prompt-Caching bei Venice und Mistral** (`supports_implicit_caching:
false`). Die cache-freundliche Blockreihenfolge aus §5 schadet nicht und zahlt
sich aus, sobald du auf ein cachendes Modell wechselst — aber sie ist bei dieser
Modellwahl kein Kostenhebel. Die Kostenrechnung oben ist bereits ohne sie.

### 11.1 Zielkonflikt: ZDR gegen Unzensiert

Bei Mistral gibt es einen eigenen Endpoint mit dem Tag `mistral/zdr` — Zero Data
Retention ist ein Anbietermerkmal, das explizit erfüllt sein muss. Venice hat nur
`venice/fp16`. Mit `zdr: true` fällt Venice also vermutlich aus dem Routing, und
Venice ist der einzige Anbieter, der Dolphin hostet: dann scheitert der Turn hart.

**Erster Test in M0**, fünf Minuten mit deinem Key. Empfohlener Ausweg, falls er
negativ ausfällt: ZDR nur für den Analysten erzwingen, beim Erzähler auf eine
Provider-Allowlist umstellen (Venice erlauben, alles andere sperren). Siehe
[OPEN-QUESTIONS.md Q8](OPEN-QUESTIONS.md).

### 11.2 Ein Anbieter, kein Failover

Dolphin läuft ausschließlich bei Venice. Deshalb ist der **Reserve-Erzähler** pro
Story konfigurierbar: bei Ausfall oder Verweigerung wechselt die Engine auf
Knopfdruck und regeneriert denselben Turn aus demselben Kontext. Weil der gesamte
Zustand in SQLite liegt und nicht im Modell, kostet ein Modellwechsel mitten in
der Kampagne nichts außer einem Stilbruch.

### 11.3 Sonstiges

- **Modell-Vergleich (M1):** derselbe deutsche Prompt, zwei Modelle nebeneinander.
  Bei kleinen Modellen ist das kein Komfort, sondern notwendig — die meisten
  Rollenspiel-Finetunes sind auf englischen Daten gemacht, und ob Dolphins
  deutsche Prosa trägt, entscheidet sich am Text, nicht an Datenblättern.
- **Kostenanzeige** pro Turn, Sitzung und Story aus dem `usage`-Objekt.
- `allow_fallbacks: false`, damit kein stiller Fallback die Filter umgeht.

## 12. Betrieb auf der DS124

Ausführlich in [BETRIEB-DS124.md](BETRIEB-DS124.md). Die Eckpunkte:

- **Zwei Wege, ein Artefakt.** Container Manager läuft auf dem Gerät, also ist ein
  arm64-Image der Hauptweg (Projekt anlegen, `docker-compose.yml` einfügen,
  starten). Dasselbe Go-Binary startet alternativ nativ über den DSM-Aufgabenplaner
  und spart dann die 100–200 MB des Docker-Daemons. Dieselbe SQLite-Datei für
  beide — Wechsel jederzeit möglich.
- **1 GB RAM, nicht erweiterbar, geteilt mit einem halben Dutzend DSM-Paketen.**
  Deshalb Go statt Python (~15 MB Image und ~25–50 MB RAM statt ~150 MB und
  ~100–150 MB), Frontend vorgebaut und eingebettet, keine lokalen Modelle.
- **Zugriff über Tailscale**, das bereits eingerichtet ist. Der Tailnet-Verkehr ist
  über WireGuard verschlüsselt, also kein Reverse Proxy und kein Zertifikat nötig.
  Der App-Login bleibt trotzdem drin.
- **Auslegung:** ein Nutzer, eine Session gleichzeitig. Die Antwortzeit bestimmt
  das Modell, nicht die NAS — dieses Programm wartet die meiste Zeit auf
  OpenRouter.
- **Umzugspfad:** dasselbe Binary läuft ohne Änderung auf einem Raspberry Pi oder
  Mini-PC, die SQLite-Datei wandert einfach mit. Erst nötig, wenn die DS124
  tatsächlich bremst.

---

## 13. Roadmap

Jeder Meilenstein ist für sich benutzbar; nach M1 kannst du spielen.

| M | Inhalt | Ergebnis |
|---|---|---|
| **M0** ✅ | Repo-Gerüst, arm64-Build in GitHub Actions (Image + Binary), `docker-compose.yml`, Auth, OpenRouter-Anbindung, **Routing-Test: ZDR + Venice** | Läuft auf der DS124, erreichbar vom Handy |
| **M1** ✅ | Chat mit Streaming, Knotenbaum, Speicherslots, Regenerate/Edit/Branch, Payload-Inspektor, Kostenanzeige, Modell-Vergleich | Spielbar, und du findest dein Modell für Deutsch |
| **M2** ✅ | Charakterkarten inkl. Grenzen und Antrieben, Persona, Prompt-Editor, Token-Budget, **Gefälligkeits-Detektor** | Dein System-Prompt trägt das Spiel, Gefälligkeit wird sichtbar |
| **M3** ✅ | Chronik, Fakten-Wiki mit Gültigkeitsspannen, Abruf über FTS5 | Figuren erinnern sich |
| **M4** | Beziehungsachsen mit Deckelung, **Widerstandsbudget, Konzessionsprüfung, Antriebsdruck**, Beziehungs-Timeline | Figuren wehren sich und wollen etwas |
| **M5** | Gruppenszenen, In-Game-Zeit, Zeitsprung-Zusammenfassung, Wissensmodell, Kontinuitätswarnungen | Die Welt hat mehr als eine Person darin |
| **M6** | PWA-Politur, Export/Import, Volltextsuche über alle Stories | Alltagstauglich |

---

## 14. Risiken

| Risiko | Gegenmaßnahme |
|---|---|
| Gefälligkeit setzt sich trotz allem durch | Vier Ebenen statt einer (§9); der Detektor macht es zumindest **sichtbar**, statt es schleichen zu lassen |
| Autonomie-Schicht überschießt: Figuren werden stur statt lebendig | Alle Schwellen einstellbar; Budget gilt pro Szene, nicht global; Direktiven verlangen *Begründung*, nicht bloßes Nein |
| Deutsche Prosa enttäuscht beim gewählten Modell | Modell-Vergleich in M1, Modellwechsel als Knopf |
| ZDR-Filter schränkt Modellauswahl spürbar ein | Bewusst akzeptiert (E7); Allowlist pro Story, damit du im Einzelfall lockern kannst |
| Kosten laufen bei langen Kampagnen weg | Budget-Manager, Caching, Utility-Routing, Monatslimit |
| Extraktor halluziniert Fakten | Provenance und Zitat bei jedem Fakt, `proposed`-Warteschlange, alles editierbar |
| Utility-Calls verdreifachen die Latenz | Detektor und Extraktion laufen **nach** dem Streaming, asynchron; du wartest nie auf sie |
| DS124 wird zu langsam oder der Speicher zu knapp | Dasselbe Binary läuft unverändert auf einem Pi oder Mini-PC; die SQLite-Datei zieht mit um |
| Ein-Personen-Projekt schläft ein | Meilensteine einzeln nutzbar; nur das Datenmodell muss von Anfang an stimmen |
