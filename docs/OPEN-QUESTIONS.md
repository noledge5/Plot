# Offene Fragen

Sortiert nach „blockiert wie viel". Q0–Q5 brauche ich, bevor sinnvoll Code
entsteht; Q6–Q16 spätestens vor dem jeweiligen Meilenstein.

---

## Q0 — Selbst bauen oder SillyTavern erweitern?

SillyTavern läuft auf einer NAS, kann OpenRouter, Charakterkarten, Lorebooks,
Streaming und hat eine Extension-API. Was es nicht kann: gedeckelte
Beziehungsachsen, ein kanonisches Fakten-Wiki mit Gültigkeitsspannen und ein
Wissensmodell pro Figur.

- Wenn dir „Karten + gutes Prompt + Vektor-Gedächtnis" reicht → SillyTavern,
  fertig in einem Abend.
- Wenn die Mechanik aus §8 der eigentliche Punkt ist → Eigenbau, weil sie sich
  nicht sauber aufpfropfen lässt.

**Meine Annahme:** Eigenbau, weil du „Beziehungsmechaniken" und „erinnernde
Charaktere" ausdrücklich als Kern nennst. Widersprich, wenn ich falsch liege.

---

## Q1 — Welches NAS-Modell genau?

Bitte: Modellnummer (z. B. DS923+), RAM, DSM-Version.

Warum es blockiert: Container Manager gibt es nicht auf jedem Modell; die
CPU-Architektur (x86 vs. ARM) entscheidet über das Docker-Image; der RAM
entscheidet, ob lokale Embeddings überhaupt in Frage kommen.

Falls kein Docker möglich ist: Fallback wäre ein Python-Prozess über den
Aufgabenplaner — machbar, aber deutlich unangenehmer zu warten.

---

## Q2 — Backend in Python oder TypeScript?

- **Python/FastAPI** (Empfehlung): stärker bei Text, Tokenizern, Embeddings,
  Retrieval-Tuning.
- **TypeScript durchgehend**: ein Sprachraum mit dem Frontend, ein Build,
  leichter selbst zu warten.

Entscheidend ist: **Willst du selbst im Code mitarbeiten?** Wenn ja, nimm die
Sprache, die du besser kannst — das schlägt jedes technische Argument.

---

## Q3 — Embeddings über OpenRouter oder lokal?

OpenRouter hat inzwischen einen echten `/api/v1/embeddings`-Endpunkt (u. a.
`openai/text-embedding-3-small`, Qwen3-Embedding), inklusive Provider-Routing und
`data_collection: "deny"`. Das heißt: ein Key, eine Rechnung, keine CPU-Last.

- **Über OpenRouter:** einfach, schnell, quasi kostenlos bei diesen Textmengen.
  Aber jeder Erinnerungsschnipsel verlässt die NAS.
- **Lokal** (fastembed/ONNX, z. B. multilingual-e5-small): nichts verlässt das
  Haus, kostet ~1–2 GB RAM und Sekunden pro Batch auf einer Celeron-CPU.
- **Vorerst ohne Vektoren:** nur FTS5/BM25. Klingt billig, trägt aber
  überraschend weit — Eigennamen und Zitate findet Stichwortsuche besser als
  Vektoren. Vektoren nachrüsten ist ein Ein-Tages-Job.

Die Frage dahinter ist eigentlich: **Wie privat müssen die Inhalte sein?** Der
Erzähltext selbst geht ohnehin an OpenRouter — wenn dich das nicht stört, sind
Embeddings dort auch kein Thema.

---

## Q4 — Was ist für dich ein Speicherslot?

- **Snapshots** wie in einem Konsolenspiel: „Speicherstand 3", laden = zurück.
- **Verzweigende Zeitlinien:** ab jedem Punkt ein What-if-Ast, mehrere Fassungen
  parallel weiterspielbar, Wechsel jederzeit.
- **Beides** (mein Vorschlag): der Baum liegt technisch ohnehin darunter, die
  UI zeigt standardmäßig nur Slots und blendet den Baum optional ein.

Beeinflusst direkt, wie der Zustand gespeichert wird — deshalb früh zu klären.

---

## Q5 — Inhalte, Modelle, Datenschutz

1. **Erzählsprache Deutsch?** Sehr relevant: deutsche Prosaqualität unterscheidet
   sich zwischen Modellen stärker als englische, und das Embedding-Modell muss
   mehrsprachig sein.
2. **Inhaltsstufe:** Erwachsenen-/explizite Inhalte? Dann fallen einige Modelle
   und Provider aus, und der Modellwechsel-Knopf wird ein Kernfeature statt eines
   Nice-to-have.
3. **Datenschutz:** soll `data_collection: "deny"` erzwungen werden? Das schließt
   Provider aus, die für Training mitloggen — kostet etwas Auswahl und manchmal
   Preis.
4. **Budget:** Größenordnung pro Monat? Bestimmt, ob Frontier-Modelle für die
   Erzählung realistisch sind oder ob der Budget-Manager scharf gestellt werden muss.

---

## Q6 — Wie viel Kontrolle über das Gedächtnis?

Neue Fakten automatisch als `canon` übernehmen (bequem, aber Halluzinationen
schleichen sich ein) oder als `proposed` in eine Prüf-Warteschlange (sauber, aber
du musst regelmäßig durchklicken)? Mischform: automatisch ab Konfidenz X, sonst
Warteschlange.

---

## Q7 — Wer erzählt?

- **Allwissender Erzähler / Spielleiter:** beschreibt Welt, Nebenfiguren, Folgen.
- **Nur die Figur antwortet:** reiner Dialog, du beschreibst deine Handlungen selbst.
- **Darf das Modell deine Figur handeln lassen?** Für viele der wichtigste
  Reibungspunkt überhaupt — als harte Regel im Prompt und optional als
  Nachprüfung, die solche Passagen markiert.

Das bestimmt die Form deines System-Prompts stärker als alles andere.

---

## Q8 — Wie viel Weltsimulation?

1. **Gruppenszenen** mit mehreren NPCs gleichzeitig, oder immer nur eine Figur?
   Gruppen sind deutlich aufwendiger: Sprecherauswahl, Turn-Reihenfolge,
   Beziehungen der NPCs untereinander.
2. **In-Game-Zeit:** sollen Tage vergehen, Termine existieren, Dinge sich zwischen
   den Szenen ändern?
3. **Off-Screen-Ereignisse:** soll die Welt weiterlaufen, während du nicht da bist?
   Sehr stimmungsvoll, aber jeder Tick kostet Tokens und kann Kanon erzeugen, den
   du nicht wolltest.

---

## Q9 — Würfel und Proben?

Rein narrativ, oder mit Werten und Zufall (Probe gelingt/misslingt, Ergebnis geht
ins Prompt)? Mechanische Fehlschläge sind ein starkes Mittel gegen die Tendenz
von Modellen, alles gelingen zu lassen — aber es ist ein eigenes Subsystem.

---

## Q10 — „Box Charaktere" — meinst du Charakterkarten?

Ich lese es als Charakterkarten. Falls ja: sollen bestehende Karten im
SillyTavern-/Chub-Format (PNG mit eingebetteten V2/V3-Metadaten) importierbar
sein? Das wäre ein guter Startvorrat, kostet aber einen Import-Mapper.

Falls du etwas anderes meinst — bitte kurz beschreiben, das ist die einzige Stelle
deiner Beschreibung, bei der ich rate.

---

## Q11 — Wie lang wird eine Kampagne?

100 Turns, 1.000 oder 10.000? Bestimmt, wie aggressiv zusammengefasst werden muss
und ob die hierarchische Ebene „Akt" überhaupt gebraucht wird.

---

## Q12 — Nur du, oder mehrere Personen?

Ein Benutzerkonto ist eine halbe Stunde Arbeit, Mehrbenutzerbetrieb mit
getrennten Stories und Rechten ein bis zwei Tage. Und: sollen mehrere Geräte
gleichzeitig in derselben Story sein (dann brauche ich Websocket-Sync statt
einfachem Polling)?

---

## Q13 — Zugang von unterwegs: Tailscale oder eigene Domain?

Tailscale ist sicherer und einfacher, verlangt aber die App auf jedem Gerät.
Reverse Proxy mit Subdomain ist bequemer im Alltag, aber du exponierst einen
Dienst ins Internet — dann brauchen wir Rate-Limiting und ordentliches
Session-Handling von Tag eins.

---

## Q14 — Sprachausgabe und Bilder?

TTS für NPC-Stimmen und Bildgenerierung für Porträts/Szenen — beides über
OpenRouter erreichbar. Mein Vorschlag: bewusst raus aus M0–M6, sonst wird nichts
fertig. Aber sag es jetzt, wenn es für dich zum Kern gehört.

---

## Q15 — Was ist dein Maßstab für „realistisch"?

Konkret: nenn mir zwei, drei Momente aus Rollenspielen (oder aus Büchern/Filmen),
die sich für dich echt angefühlt haben — und zwei, die dich rausgeworfen haben.
Daraus baue ich die Regeln im Stilleitfaden und die Beispiele im Prompt. Das ist
die Frage, die am meisten über die Qualität des Ergebnisses entscheidet, und die
einzige, die ich nicht aus der Technik ableiten kann.

---

## Q16 — Wie weiter?

Soll ich M0 + M1 direkt bauen (lauffähiger Container mit Chat, Streaming, Slots
und Payload-Inspektor), sobald Q1–Q5 beantwortet sind? Oder erst den Plan
festzurren und dann in einem Rutsch?
