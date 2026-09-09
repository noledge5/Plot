# Modell-Auswahl

**Stand der Daten: 9. September 2026**, direkt aus `openrouter.ai/api/v1/models`
und den Endpoint-Abfragen. Preise pro 1 Mio. Token. Das veraltet — die Zahlen
sind zum Planen da, nicht zum Zementieren; die Engine liest den Katalog live.

Grundsatz (E12): kleine, günstige Modelle. Das ist eine gute Entscheidung, sie
verschiebt aber den Engpass — siehe unten.

---

## 1. Drei Rollen, nicht zwei

| Rolle | Aufgabe | Anforderung |
|---|---|---|
| **Erzähler** | die Prosa | unzensiert, guter deutscher Stil, 8k+ Ausgabe |
| **Analyst** | Extraktion, Gefälligkeits-Detektor, Zusammenfassungen, Konzessionsstufen | **striktes JSON-Schema**, sehr billig, **muss den erzeugten Text sehen dürfen** |
| **Reserve-Erzähler** | Ausfall oder Verweigerung | dasselbe wie Erzähler, anderer Anbieter |

Der Analyst ist eine eigene Rolle geworden, weil zwei Befunde das erzwingen:

**Dein Wunschmodell kann kein striktes JSON.**
`cognitivecomputations/dolphin-mistral-24b-venice-edition` (auf OpenRouter als
„Venice: Uncensored") listet `response_format`, aber **nicht**
`structured_outputs`. Die ganze Zustandsmechanik aus PLAN.md §7 und §9 hängt an
schema-validiertem JSON. Auf dem Erzählmodell ist das nicht zu haben — also läuft
es auf einem zweiten, und der Erzähler macht nur, was er gut kann: erzählen.

**Der Analyst liest den Erzähltext mit.** Er muss also dieselben Inhalte
verarbeiten dürfen wie der Erzähler. Ein moderiertes Utility-Modell verweigert
sonst genau dann, wenn es gebraucht wird. Das schließt Google für explizite Stoffe
praktisch aus, obwohl OpenRouter Gemini als `is_moderated: false` führt — Googles
eigene Filter greifen trotzdem.

---

## 2. Der Zielkonflikt, den du entscheiden musst

**Zero Data Retention (E7) und Venice schließen sich vermutlich aus.**

Bei Mistral führt OpenRouter einen eigenen Endpoint mit dem Tag `mistral/zdr` —
ZDR ist also ein Provider-Merkmal, das ein Anbieter explizit erfüllt. Venice hat
nur `venice/fp16`, keinen ZDR-Endpoint. Setzt die Engine `zdr: true`, fällt Venice
mit hoher Wahrscheinlichkeit aus dem Routing — und Venice ist der **einzige**
Anbieter, der Dolphin hostet. Dann kommt kein Endpoint zurück, und der Turn
scheitert hart.

Das ist in fünf Minuten mit deinem Key zu klären und der **erste Test in M0**.
Fällt er negativ aus, hast du drei Wege:

1. ZDR nur für den Analysten erzwingen, beim Erzähler auf Provider-Allowlist
   umstellen (Venice erlauben, alles andere sperren).
2. Auf ein unzensiertes Modell wechseln, das bei einem ZDR-fähigen Anbieter läuft.
3. ZDR fallenlassen und stattdessen auf die Datenrichtlinie von Venice vertrauen.

Meine Empfehlung ist Weg 1: der Analyst sieht denselben Text, aber er ist der
Teil, den man leichter tauschen kann.

---

## 3. Erzähler-Kandidaten für den A/B-Test in M1

Deutsch ist das offene Risiko bei allen: die meisten Rollenspiel-Finetunes sind
auf englischen Daten gemacht. Dolphin basiert laut OpenRouter-Beschreibung auf
`Mistral-Small-24B-Instruct-2501` — die Basis kann Deutsch ordentlich, der
Finetune darüber ist die Unbekannte. Genau deshalb der Vergleich.

| Modell | in / out | Kontext | Anmerkung |
|---|---|---|---|
| `cognitivecomputations/dolphin-mistral-24b-venice-edition` | $0.20 / $0.90 | 128k | dein Kandidat; nur Venice, kein strict JSON, kein Caching, max. 8k Ausgabe |
| `mistralai/mistral-small-2603` | $0.15 / $0.60 | 262k | auch **über Venice** verfügbar, strict JSON, europäischer Anbieter, ZDR-Endpoint vorhanden |
| `thedrummer/cydonia-24b-v4.1` | $0.30 / $0.50 | 131k | Rollenspiel-Finetune, strict JSON |
| `sao10k/l3.3-euryale-70b` | $0.65 / $0.75 | 131k | größer, gilt als stark im Rollenspiel |
| `google/gemini-2.5-flash-lite` | $0.10 / $0.40 | 1M | „Gemini Flash", sehr billig und stark im Deutschen — aber Googles Filter |

`mistralai/mistral-small-2603` ist der interessanteste Vergleichspunkt: dieselbe
Modellfamilie wie Dolphin, aber mit striktem JSON, mehreren Anbietern (Failover!),
einem ZDR-Endpoint — und über den Venice-Endpoint vermutlich ebenfalls ungefiltert.
Wenn dessen deutsche Prosa reicht, löst ein Modell drei Probleme auf einmal.

## 4. Analyst-Kandidaten

| Modell | in / out | strict JSON | Anmerkung |
|---|---|---|---|
| `mistralai/mistral-nemo` | $0.019 / $0.030 | ja | mit Abstand am billigsten, 131k |
| `mistralai/mistral-small-2603` (Venice) | $0.15 / $0.60 | ja | wenn der Analyst denselben Stoff sehen können muss |
| `deepseek/deepseek-v4-flash` | $0.07 / $0.14 | ja | 1M Kontext |
| `meta-llama/llama-3.1-8b-instruct` | $0.05 / $0.08 | ja | |

---

## 5. Was ein Turn tatsächlich kostet

Rechnung mit 6.000 Token Prompt, 600 Token Antwort, zwei Analyst-Calls
(Extraktion + Detektor) à 3.000 / 300 Token, **ohne** Prompt-Caching — Venice und
Mistral bieten keins (`supports_implicit_caching: false`):

```
Erzähler   Dolphin       6.000 × $0.20/M  +  600 × $0.90/M   = $0.00174
Analyst    Mistral Nemo  2 × (3.000 × $0.019/M + 300 × $0.03/M) = $0.00013
Embeddings                                                     ≈ $0.00000
─────────────────────────────────────────────────────────────────────────
pro Turn                                                       ≈ $0.0019
1.000 Turns                                                    ≈ $1.90
```

**Das ist die eigentliche Nachricht: Geld ist bei dieser Modellklasse kein
Engpass.** Eine ganze Kampagne kostet weniger als ein Kaffee. Der Budget-Manager
aus PLAN.md §5 bleibt drin, aber er dient ab jetzt der *Qualität* — kurze Prompts
für bessere Regelbefolgung —, nicht dem Sparen.

Zwei Nebenwirkungen davon:

- Die Analyst-Calls sind so billig, dass **mehr davon** sinnvoll ist. Der
  Gefälligkeits-Detektor darf großzügig laufen, ein zweiter Prüfpass ist kein
  Kostenthema mehr.
- Ein **automatischer Neuversuch** bei Detektor-Befund (statt nur zu markieren)
  kostet einen weiteren Erzähler-Call, also 0,17 Cent. Das macht die harte
  Variante — „bei Muster 7 immer automatisch neu" — praktisch gratis. Bei einem
  teuren Modell hätte ich davon abgeraten.

---

## 6. Betriebsrisiko: ein einziger Anbieter

Dolphin läuft ausschließlich bei Venice. Kein Failover, keine Preiskonkurrenz,
und wenn Venice das Modell zurückzieht, steht deine laufende Kampagne. Die Uptime
lag bei der Abfrage bei 99,99 % (24 h), das ist nicht das Problem — die
Abhängigkeit ist es.

Deshalb: **Reserve-Erzähler pro Story konfigurierbar.** Fällt der primäre aus oder
verweigert er, wechselt die Engine auf Knopfdruck und regeneriert denselben Turn
aus demselben Kontext. Da der ganze Zustand in SQLite liegt und nicht im Modell,
kostet ein Modellwechsel mitten in der Kampagne nichts außer einem Stilbruch.
