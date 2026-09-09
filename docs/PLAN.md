# Plot — Architektur- und Umsetzungsplan

Lokale Rollenspiel-Engine mit eigenem System-Prompt, Charakterkarten, Beziehungs-
und Gedächtnismechanik. Läuft als ein Docker-Container auf einer Synology NAS,
bedienbar von jedem Gerät im Haushalt (Browser/PWA).

> Status: Entwurf. Offene Entscheidungen siehe [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md).
> Punkte, die dort noch offen sind, sind hier mit **[Q<n>]** markiert.

---

## 1. Leitprinzipien

Diese fünf Sätze entscheiden später jeden Zweifelsfall:

1. **Der System-Prompt gehört dir.** Die Engine injiziert niemals versteckten Text.
   Alles, was ins Modell geht, steht entweder in deinem Template oder in einem
   Platzhalter, den du selbst gesetzt hast. Es gibt einen Inspektor, der den
   exakten Request zeigt.
2. **Zustand ist Daten, nicht Prosa.** Beziehungen, Fakten und Wissen liegen
   strukturiert in SQLite. Das LLM erzählt; es verwaltet den Zustand nicht.
3. **Das Modell darf vorschlagen, die Engine entscheidet.** Zustandsänderungen
   kommen als JSON-Vorschlag zurück und werden gedeckelt, geloggt und sind
   nachträglich editierbar.
4. **Nichts geht verloren.** Jeder Turn ist ein Knoten in einem Baum. Regenerieren
   erzeugt einen Zweig, löscht nichts.
5. **Ein Container, keine Fremddienste.** Kein Postgres, kein Redis, keine separate
   Vektordatenbank. Was auf einer NAS nicht mit 2 GB RAM läuft, kommt nicht rein.

---

## 2. Warum nicht einfach SillyTavern?

SillyTavern kann Charakterkarten, Lorebooks und Streaming — und läuft auf einer NAS.
Die drei Dinge, die es *nicht* kann und die dein Ziel („realistisch, erinnernd,
kohärent") ausmachen:

| Fehlt in SillyTavern | Was Plot stattdessen tut |
|---|---|
| Beziehungen sind Prosa im Charakterblatt, driften beliebig | Gerichtete Achsen mit gedeckelten Deltas, Verlauf und Begründung pro Turn |
| Gedächtnis = Vektorsuche über rohe Chatlogs | Kanonisches Fakten-Wiki mit Herkunft, Gültigkeitsspanne und Widerspruchserkennung |
| Jeder NPC weiß implizit alles, was im Kontext steht | Wissensmodell: Fakten sind an Figuren gebunden, Prompt wird pro Sprecher gefiltert |

Wenn dir Punkt 1 und 2 reichen, ist SillyTavern + ein gutes Prompt der schnellere
Weg — das ist eine ehrliche Alternative und in OPEN-QUESTIONS.md als Frage 0 notiert.

---

## 3. Systemüberblick

```
   Handy / Laptop / Tablet
            │  HTTPS (Tailscale oder DSM Reverse Proxy)
            ▼
   ┌─────────────────────────────────────────────┐
   │  Docker-Container auf der Synology NAS      │
   │                                             │
   │   React-PWA  ──►  API (FastAPI)             │
   │                     │                       │
   │                     ├─ Prompt-Builder ──────┼──► OpenRouter
   │                     │   (Budget, Slots)     │     /chat/completions (Erzählung)
   │                     ├─ State-Extractor      │     /chat/completions (Utility, billig)
   │                     ├─ Memory-Indexer       │     /embeddings        (RAG)
   │                     │                       │
   │                     ▼                       │
   │                  SQLite (WAL)               │
   │                  + FTS5 + sqlite-vec        │
   └─────────────────────────────────────────────┘
                         │
                  /data  Volume  ──► Hyper Backup
```

Ein Prozess, eine Datei, ein Volume. Kein Zustand außerhalb von `/data`.

---

## 4. Stack **[Q2]**

**Empfehlung: Python 3.12 + FastAPI, React/Vite/TypeScript im Frontend.**

- **Backend:** FastAPI + `httpx` (SSE-Streaming) + SQLite via `sqlite3`/SQLModel.
  Grund: Textverarbeitung, Tokenizer, lokale Embedding-Runtimes und
  Schema-validiertes JSON sind in Python schlicht ausgereifter, und der Teil,
  der später wehtut (Retrieval-Tuning, Fakten-Dedup), lebt genau dort.
- **Frontend:** React + Vite + TypeScript + Tailwind, als statisches Bundle vom
  Backend ausgeliefert. Streaming über SSE. PWA-Manifest für „Zum Homescreen".
- **DB:** SQLite im WAL-Modus. FTS5 (in jedem SQLite enthalten) für Stichwortsuche,
  `sqlite-vec` als Extension für Vektoren. Beides in derselben Datei — ein Backup
  sichert alles.
- **Alternative:** komplett TypeScript (Node/Hono + React). Ein Sprachraum, ein
  Build, einfacher zu warten, wenn du selbst mitcodest. Kostet bei Embeddings
  und Tokenizern etwas Bequemlichkeit.

---

## 5. Datenmodell

Kern-Idee: **Event-Sourcing auf einem Baum.** Der Zustand (Beziehungen, gültige
Fakten) wird nicht gespeichert, sondern aus dem Pfad von der Wurzel zum aktuellen
Knoten gefaltet. Dadurch sind Speicherslots und Verzweigungen automatisch korrekt —
ein Zweig kann keinen Zustand aus einem anderen Zweig sehen. Materialisierte
Snapshots dienen nur als Cache.

```sql
story            (id, title, created_at, settings_json)
prompt_template  (id, story_id, kind, name, body, version, created_at)
                 -- kind: narrator | extractor | summarizer | ...

persona          (id, story_id, name, sheet_json)          -- deine Figur(en)
character        (id, story_id, name, sheet_json, avatar_path, volatility_json)
                 -- sheet_json: Rolle, Werte, Sprechweise, Ziele, Tabus, Geheimnisse

scene            (id, story_id, location, ingame_time, present_character_ids)

node             (id, story_id, parent_id, kind, role, content,
                  scene_id, model, usage_json, created_at)
                 -- kind: turn | narration | event | offscreen | note
                 -- Baum: parent_id -> node.id

save_slot        (id, story_id, node_id, name, kind, created_at, preview)
                 -- kind: manual | auto | checkpoint

memory_fact      (id, story_id, subject_id, kind, text, importance, confidence,
                  status, valid_from_node, valid_to_node, source_node_id, pinned)
                 -- status: proposed | canon | retired
memory_vec       (fact_id, embedding)                       -- sqlite-vec
memory_fts       (fact_id, text)                            -- FTS5
memory_summary   (id, story_id, level, from_node, to_node, text)
                 -- level: 1=Szene, 2=Kapitel, 3=Akt

lore_entry       (id, story_id, keys_json, text, priority, always_on)

knowledge        (character_id, fact_id, since_node, how)   -- wer weiß was
                 -- how: witnessed | told | inferred | assumed(falsch!)

rel_delta        (id, node_id, from_char, to_char, axis, delta, reason, quote)
rel_snapshot     (node_id, from_char, to_char, axis, value)  -- Cache

run_log          (id, node_id, purpose, request_json, response_meta_json,
                  cost_usd, latency_ms)
```

`run_log` ist nicht optional: ohne den exakten abgeschickten Payload kannst du nie
beurteilen, ob eine schlechte Antwort am Modell oder an deinem Prompt lag.

---

## 6. Der Prompt-Builder (Herzstück)

Du schreibst den System-Prompt selbst. Die Engine rendert ihn mit Platzhaltern:

```
{{persona}}            deine Figur
{{characters}}         Blätter der anwesenden NPCs (gefiltert nach Sprecher)
{{relationships}}      Beziehungslage als Verhaltensbeschreibung, nicht als Zahlen
{{world}}              statische Weltbeschreibung
{{lore}}               per Stichwort getriggerte Lore-Einträge
{{memories}}           abgerufene Fakten aus dem Wiki
{{summary}}            hierarchische Zusammenfassung des bisherigen Verlaufs
{{scene}}              Ort, Zeit, Anwesende, was zuletzt geschah
{{style}}              dein Stilleitfaden
{{author_note}}        Regieanweisung, wird kurz vor dem letzten Turn eingefügt
```

Regeln des Builders:

- **Reihenfolge ist fix und cache-freundlich:** stabile Blöcke zuerst
  (System-Prompt, Charaktere, Welt, Lore), volatile zuletzt (Memories, Szene,
  Verlauf). Das maximiert Prompt-Cache-Treffer; bei Anthropic/Qwen setzt der
  Builder automatisch `cache_control`-Breakpoints an die Grenze, bei
  OpenAI/DeepSeek/Gemini passiert es implizit. Ergebnis: der teure Präfix wird
  bei jedem Turn nur einmal voll bezahlt.
- **Budget-Manager:** jeder Block bekommt ein Token-Kontingent und eine Priorität.
  Wird es eng, wird von unten gekürzt: älteste wörtliche Turns wandern zuerst in
  die Zusammenfassung. Was gekürzt wurde, steht im Inspektor.
- **Zahlen werden übersetzt.** `trust=22` geht nicht als Zahl ins Prompt, sondern
  als Verhaltensregel: *„Mira misstraut dir. Sie weicht persönlichen Fragen aus
  und prüft, ob deine Aussagen zu dem passen, was sie schon weiß."* Modelle
  befolgen Verhaltensbeschreibungen deutlich zuverlässiger als Skalen.
- **Sprecherfilter:** vor dem Rendern wird `{{memories}}` und `{{characters}}` auf
  das gefiltert, was die sprechende Figur wissen darf (siehe §8).

---

## 7. Gedächtnis

Vier Ebenen, bewusst getrennt:

| Ebene | Inhalt | Erzeugung |
|---|---|---|
| L0 | letzte N Turns wörtlich | — |
| L1 | Szenen-/Kapitel-/Akt-Zusammenfassung | billiges Modell, rollierend |
| L2 | Fakten-Wiki: atomare, editierbare Aussagen mit Herkunft | Extraktion + deine Korrektur |
| L3 | Lorebook: statisches Weltwissen, per Stichwort | von dir geschrieben |

**Retrieval ist hybrid, nicht nur Vektoren.** Reine Vektorsuche versagt bei genau
den Fragen, die im Rollenspiel zählen (Eigennamen, Daten, „was hat er damals
gesagt"). Score:

```
score = w1·cosine + w2·bm25 + w3·importance + w4·recency + w5·betrifft_anwesende
immer dabei: pinned facts, Fakten über anwesende Figuren, offene Konflikte
```

**Extraktion läuft asynchron** nach dem Turn, blockiert die Antwort also nicht.
Ein Utility-Call mit striktem JSON-Schema liefert Fakt-Kandidaten. Danach:

1. **Dedup:** Ähnlichkeit über Schwelle → bestehenden Fakt aktualisieren statt neu.
2. **Widerspruch:** neuer Fakt widerspricht altem → alter bekommt `valid_to_node`
   und Status `retired`, statt gelöscht zu werden. Damit bleibt „sie *war* Ärztin,
   bis sie gekündigt hat" korrekt darstellbar.
3. **Review:** neue Fakten landen je nach Einstellung als `proposed` in einer
   Warteschlange oder direkt als `canon` **[Q6]**.

---

## 8. Realismus-Mechanik

Vier Bausteine, in dieser Reihenfolge nach Wirkung pro Aufwand:

### 8.1 Beziehungsachsen (gerichtet)

Sympathie ist nicht symmetrisch — deshalb pro Richtung (A→B ≠ B→A):

`trust, warmth, attraction, respect, familiarity, tension, resentment, obligation, fear`

Werte −100…+100. Jede Figur hat eine `volatility` pro Achse: eine misstrauische
Figur baut Vertrauen langsam auf und verliert es schnell — das allein erzeugt
schon spürbar unterschiedliche Persönlichkeiten.

### 8.2 Zustands-Extraktion mit Deckelung

Nach jedem Turn ein billiger Utility-Call mit `response_format: json_schema`:

```json
{"deltas":[{"from":"mira","to":"player","axis":"trust","delta":-6,
            "reason":"Er hat ihre Frage nach dem Brief ausweichend beantwortet.",
            "quote":"…"}]}
```

Die Engine **clampt** (z. B. max ±5 pro Turn, achsen- und figurenabhängig), loggt
Begründung samt Zitat und schreibt einen `rel_delta`. Du kannst jeden Delta im UI
nachträglich korrigieren — der Zustand wird neu gefaltet. Ohne Deckelung springen
Modelle nach einem einzigen netten Satz von „Misstrauen" auf „Liebe"; das ist der
häufigste Realismus-Killer.

### 8.3 Wissensmodell (der größte Hebel)

Jeder Fakt hat eine `known_by`-Menge mit *wie* er bekannt wurde. Beim Prompt-Bau
wird nach Sprecher gefiltert: Was Mira nicht miterlebt hat und ihr niemand erzählt
hat, steht nicht in ihrem Kontext. Das erzeugt von allein die Momente, die sich
echt anfühlen — Missverständnisse, Nachfragen, jemand erfährt etwas zu spät.
Der Modus `assumed` erlaubt sogar bewusst falsche Annahmen einer Figur.

### 8.4 Agenda, Zeit und Reibung

- Jeder NPC hat 1–3 aktive Ziele und Bedürfnisse (Geld, Sicherheit, Anerkennung …).
  Sie stehen im Prompt und werden vom Extraktor fortgeschrieben.
- **Off-Screen-Ticks [Q8]:** vergeht In-Game-Zeit, generiert ein Utility-Call, was
  bei den Figuren passiert ist, als `offscreen`-Knoten. Beim nächsten Treffen ist
  das Kontext. Das ist der Unterschied zwischen einer Welt und einer Warteschleife.
- **Drift:** ohne Kontakt bewegen sich `warmth` und `tension` langsam Richtung
  Baseline.
- **Reibungsregeln** gehören in deinen Stilleitfaden: NPCs dürfen ablehnen, lügen,
  eigene Interessen verfolgen und das Gespräch abbrechen. Ohne diese explizite
  Erlaubnis wird jedes Modell gefällig — kein Mechanismus kompensiert das.

### 8.5 Kontinuitätsprüfung

Optionaler Pass nach der Antwort: prüft sie gegen `canon`-Fakten und markiert
Widersprüche als Warnung im UI. **Nicht** automatisch umschreiben — sonst
korrigierst du hinterher die Korrektur.

---

## 9. Speicherslots und Verzweigung **[Q4]**

Weil jeder Turn ein Knoten mit `parent_id` ist:

- **Regenerieren** = Geschwisterknoten, alte Version bleibt erhalten.
- **Bearbeiten** eines alten Turns = neuer Zweig ab dort. Fakten mit
  `valid_from_node` außerhalb des neuen Pfads sind dort automatisch ungültig — das
  Gedächtnis „vergisst" korrekt, was in der verworfenen Zeitlinie passierte.
- **Speicherslot** = benannter Zeiger auf einen Knoten. Laden heißt: Zeiger setzen.
  Kein Kopieren, kein Datenverlust, beliebig viele Slots.
- **Autosave** alle N Turns als `kind=auto`, mit Rotation.
- **Export** einer Story als `.zip` (SQLite-Auszug + JSON + Bilder) für Backup
  und zum Weitergeben.

---

## 10. Kosten und Modell-Routing

- **Erzählung:** starkes Modell, hier lohnt es sich.
- **Utility** (Extraktion, Zusammenfassung, Titel, Off-Screen): billiges,
  schnelles Modell. Machen 80 % der Calls aus, aber sollen <10 % der Kosten sein.
- **Prompt-Caching:** Blockreihenfolge wie in §6, `session_id` setzen, damit
  OpenRouter sticky zum selben Provider routet. Bei langen Charakter-/Lore-Blöcken
  ist das der größte Einzelhebel auf die Rechnung.
- **Anzeige:** Kosten pro Turn, pro Sitzung und pro Story direkt aus dem
  `usage`-Objekt der Antwort; Monatsbudget mit Warnschwelle.
- **Provider-Routing:** `provider.data_collection: "deny"` erzwingbar, plus
  Allow-/Blocklisten pro Story **[Q5]**.

---

## 11. Betrieb auf der Synology **[Q1]**

- **Image:** ein Multi-Arch-Image (amd64 + arm64), `docker-compose.yml`, ein
  Volume `/data`. Installation über Container Manager (DSM 7.2+) oder per SSH.
  Container Manager gibt es nicht auf allen Modellen — überwiegend x86/Plus-Serie
  und einige ARM-Modelle; das ist der erste zu klärende Punkt.
- **Zugriff von unterwegs:** **Tailscale** (Paket im Synology Package Center) ist
  die empfohlene Variante — kein offener Port, kein Zertifikatsgefummel, läuft auf
  iOS/Android. Alternative: DSM Reverse Proxy mit eigener Subdomain und Let's
  Encrypt. **Kein Port-Forwarding auf den Container ohne Auth.**
- **Auth:** Passwort (argon2) + Session-Cookie, optional TOTP. Der OpenRouter-Key
  liegt ausschließlich serverseitig in einer `.env`, nie im Frontend-Bundle.
- **Backup:** Hyper Backup auf `/data`; zusätzlich App-eigener Story-Export.
  SQLite im WAL-Modus vorher per `VACUUM INTO` konsistent wegschreiben.
- **Ressourcen:** Das Backend ist I/O-lastig, nicht CPU-lastig — solange Embeddings
  extern laufen. Lokale Embeddings brauchen ~1–2 GB RAM und belasten eine
  Celeron-CPU spürbar (Sekunden pro Batch, aber asynchron, also erträglich) **[Q3]**.

---

## 12. Roadmap

Jeder Meilenstein ist für sich benutzbar — du kannst nach M1 schon spielen.

| M | Inhalt | Ergebnis |
|---|---|---|
| **M0** | Repo-Gerüst, Docker-Image, Auth, OpenRouter-Anbindung, Modell-Liste | Läuft auf der NAS, erreichbar vom Handy |
| **M1** | Chat mit Streaming, Knotenbaum, Speicherslots, Regenerate/Edit/Branch, Payload-Inspektor, Kostenanzeige | Spielbar |
| **M2** | Charakterkarten, Persona, Prompt-Template-Editor mit Platzhaltern, Token-Budget | Dein eigener System-Prompt trägt das Spiel |
| **M3** | Zusammenfassungen, Fakten-Wiki, Hybrid-Retrieval, Lorebook | Figuren erinnern sich |
| **M4** | Beziehungsachsen, Delta-Extraktion mit Deckelung, Beziehungs-Timeline im UI | Beziehungen entwickeln sich nachvollziehbar |
| **M5** | Wissensmodell, NPC-Agenda, Off-Screen-Ticks, Kontinuitätswarnungen | Die Welt lebt weiter |
| **M6** | PWA-Politur, Export/Import, Volltextsuche über alle Stories, Gruppenszenen | Alltagstauglich |

---

## 13. Risiken und Gegenmaßnahmen

| Risiko | Gegenmaßnahme |
|---|---|
| Kontext-Drift trotz Gedächtnis | Harte `canon`-Fakten immer im Prompt, regelmäßiges Re-Anchoring, Kontinuitätswarnungen |
| Kosten laufen bei langen Kampagnen weg | Budget-Manager, aggressive Summarisierung, Caching, Utility-Routing, Monatslimit |
| Modell verweigert Inhalte oder bricht Ton | Modell-/Provider-Auswahl pro Story, schneller Modellwechsel mitten im Spiel **[Q5]** |
| Extraktor halluziniert Fakten | Provenance + Zitat bei jedem Fakt, `proposed`-Warteschlange, alles editierbar |
| Over-Engineering | Wissensmodell und Agenda erst ab M5; M1–M4 sind ohne sie vollständig nutzbar |
| Ein-Personen-Projekt schläft ein | Meilensteine einzeln nutzbar, kein „Big Bang"; Datenmodell ist der einzige Teil, der von Anfang an stimmen muss |
