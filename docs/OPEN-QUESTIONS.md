# Offene Fragen

Beantwortet ist beantwortet — siehe [ENTSCHEIDUNGEN.md](ENTSCHEIDUNGEN.md).
Hier steht nur noch, was fehlt.

---

## Blockiert den Start

### Q1 — Welches NAS-Modell genau?

Ich brauche: **Modellnummer, RAM, DSM-Version.**
Zu finden unter DSM → Systemsteuerung → Info-Center → Allgemein.

Warum es blockiert: Container Manager gibt es nicht auf jedem Synology-Modell,
und die CPU-Architektur entscheidet über das Docker-Image. Ich baue zwar
Multi-Arch, aber wenn dein Modell gar kein Docker kann, brauchen wir einen
anderen Weg — dann läuft es über den DSM-Aufgabenplaner oder auf einem anderen
Gerät im Netz.

Zweite Hälfte davon: **Zugang von unterwegs — Tailscale oder eigene Domain?**
Tailscale ist sicherer und in zehn Minuten eingerichtet, verlangt aber die App auf
jedem Gerät. Reverse Proxy mit Subdomain ist bequemer, exponiert aber einen Dienst
ins Internet; dann brauche ich von Tag eins Rate-Limiting und härteres
Session-Handling. Meine Empfehlung ist Tailscale.

### Q2 — Budget

Größenordnung pro Monat für OpenRouter? Davon hängt ab, ob ein Spitzenmodell für
jeden Erzähl-Turn realistisch ist oder ob der Budget-Manager von Anfang an scharf
gestellt wird. Eine Hausnummer reicht — 5 €, 20 €, 100 € sind drei sehr
verschiedene Architekturen im Detail.

### Q8 — ZDR oder Dolphin? (neu)

Zero Data Retention (E7) und dein Wunschmodell schließen sich vermutlich aus:
Dolphin läuft nur bei Venice, und Venice hat keinen ZDR-Endpoint. Details in
[modelle.md §2](modelle.md).

Ich kläre das im ersten Test in M0 mit deinem Key. Falls es sich bestätigt, ist
meine Empfehlung: **ZDR nur für den Analysten erzwingen**, beim Erzähler auf eine
Provider-Allowlist umstellen (nur Venice erlaubt). Sag mir, ob du das mitträgst
oder ob ZDR für dich überall gelten muss — im zweiten Fall suchen wir ein anderes
unzensiertes Modell.

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
