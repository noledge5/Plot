# Offene Fragen

Beantwortet ist beantwortet — siehe [ENTSCHEIDUNGEN.md](ENTSCHEIDUNGEN.md).
Hier steht nur noch, was fehlt.

---

## Beantwortet

- **Q1 Hardware:** DS124, 1 GB RAM, DSM 7.3.2, Tailscale eingerichtet → E14, E15.
  Kein Docker; Betriebskonzept in [BETRIEB-DS124.md](BETRIEB-DS124.md).
- **Q2 Budget:** erledigt. Bei rund 0,19 Cent pro Turn (modelle.md §5) ist Geld
  kein Engpass mehr.
- **Q8 ZDR:** ZDR für den Analysten, Provider-Allowlist für den Erzähler → E16.

---

---

## Vor M2

### Q3 — Was ist dein Maßstab für „realistisch"?

Die wichtigste Frage im ganzen Dokument, und die einzige, die ich nicht aus der
Technik ableiten kann.

Nenn mir zwei, drei Momente — aus Rollenspielen, Büchern, Serien —, die sich für
dich **echt** angefühlt haben. Und zwei, die dich rausgeworfen haben. Aus dem
Kontrast baue ich die Regeln im Stilleitfaden, die Muster des
Gefälligkeits-Detektors und die Beispiele im Prompt.

Wenn du nur eine Frage beantwortest, dann diese.

### Q4 — „Box Charaktere" — meinst du Charakterkarten?

Ich habe es als Charakterkarten gelesen und so geplant. Falls ja: sollen
bestehende Karten im SillyTavern-/Chub-Format (PNG mit eingebetteten
V2/V3-Metadaten) importierbar sein? Guter Startvorrat, kostet einen Import-Mapper
— und die Grenzen- und Antriebsfelder aus §9 fehlen dort, die müsstest du
nachtragen.

Falls du etwas anderes gemeint hast: das ist die einzige Stelle deiner
Beschreibung, bei der ich rate.

### Q5 — Wie viel Kontrolle über das Gedächtnis?

Neue Fakten automatisch als `canon` (bequem, Halluzinationen schleichen sich ein)
oder als `proposed` in eine Prüf-Warteschlange (sauber, du musst durchklicken)?
Mischform: automatisch ab Konfidenz X, darunter Warteschlange. Ich würde mit der
Mischform starten.

---

## Vor M5

### Q6 — Wie lang wird eine Kampagne?

100 Turns, 1.000 oder 10.000? Bestimmt, wie aggressiv zusammengefasst werden muss
und ob die Ebene „Akt" überhaupt gebraucht wird.

### Q7 — Nur du, oder mehrere Personen?

Ein Konto ist eine halbe Stunde Arbeit, echter Mehrbenutzerbetrieb ein bis zwei
Tage. Und: sollen mehrere Geräte **gleichzeitig** in derselben Story sein? Dann
braucht es Websocket-Sync statt einfachem Nachladen.

---

## Bewusst zurückgestellt

- **Sprachausgabe und Bildgenerierung** (TTS für Stimmen, Porträts, Szenenbilder).
  Über OpenRouter erreichbar, aber raus aus M0–M6 — sonst wird nichts fertig.
  Sag Bescheid, wenn das für dich zum Kern gehört.
- **Off-Screen-Simulation** — ersetzt durch die Zeitsprung-Zusammenfassung (E11).
- **Würfel und Proben** — ersetzt durch die Autonomie-Schicht (E10).

---

## Und dann?

Sobald Q1 und Q2 beantwortet sind, kann ich **M0 und M1** bauen: lauffähiger
Container auf deiner NAS, Chat mit Streaming, Speicherslots, Payload-Inspektor,
Kostenanzeige und den Modell-Vergleich für Deutsch. Danach hast du etwas, mit dem
du spielen und urteilen kannst — und Q3 beantwortet sich beim Spielen leichter als
am Schreibtisch.
