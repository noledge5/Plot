import { useEffect, useState } from "react";
import { hole, schicke } from "../api";
import type { Einstellungen as Werte, Modell, Zustand } from "../types";
import ModellWahl from "./ModellWahl";
import { Dialog, Feld, Hinweis, Knopf, eingabeKlasse } from "./ui";

type Testergebnis = { rolle: string; modell: string; ok: boolean; anbieter?: string; fehler?: string };

export default function Einstellungen({
  offen,
  schliessen,
  keyGesetzt,
  geaendert,
}: {
  offen: boolean;
  schliessen: () => void;
  keyGesetzt: boolean;
  geaendert: () => void;
}) {
  const [werte, setWerte] = useState<Werte | null>(null);
  const [key, setKey] = useState("");
  const [modelle, setModelle] = useState<Modell[]>([]);
  const [fehler, setFehler] = useState("");
  const [gespeichert, setGespeichert] = useState(false);
  const [tests, setTests] = useState<Testergebnis[]>([]);
  const [testLaeuft, setTestLaeuft] = useState(false);
  const [zustand, setZustand] = useState<Zustand | null>(null);

  useEffect(() => {
    if (!offen) return;
    hole<{ settings: Werte }>("/api/settings")
      .then((d) => setWerte(d.settings))
      .catch((e) => setFehler(String(e.message ?? e)));
    hole<Zustand>("/api/state")
      .then(setZustand)
      .catch(() => setZustand(null));
  }, [offen]);

  // Der Katalog kommt erst, wenn ein Schlüssel hinterlegt ist.
  useEffect(() => {
    if (!offen || !keyGesetzt) return;
    hole<Modell[]>("/api/models")
      .then(setModelle)
      .catch(() => setModelle([]));
  }, [offen, keyGesetzt]);

  if (!offen) return null;

  const setzen = (teil: Partial<Werte>) => setWerte((w) => (w ? { ...w, ...teil } : w));

  // Die Allowlist gilt für jedes Erzählmodell. Wer das Modell wechselt und die
  // Liste stehen lässt, bekommt beim nächsten Zug "kein Anbieter gefunden" -
  // ein Fehler, in den man genau einmal tappt und dann jedes Mal wieder.
  const anbieterKonflikt =
    !!werte &&
    werte.narratorProviders.length > 0 &&
    !!werte.narratorModel &&
    !werte.narratorProviders.some((p) =>
      werte.narratorModel.toLowerCase().includes(p.toLowerCase().split("/")[0]),
    ) &&
    !werte.narratorModel.toLowerCase().includes("dolphin");

  async function speichern() {
    if (!werte) return;
    setFehler("");
    try {
      await schicke("/api/settings", { settings: werte, key: key || undefined }, "PUT");
      setKey("");
      setGespeichert(true);
      setTimeout(() => setGespeichert(false), 2500);
      geaendert();
      if (!modelle.length) hole<Modell[]>("/api/models").then(setModelle).catch(() => {});
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function routingTesten() {
    setTestLaeuft(true);
    setTests([]);
    try {
      const rollen = ["narrator", "reserve", "analyst"];
      const ergebnisse: Testergebnis[] = [];
      for (const rolle of rollen) {
        ergebnisse.push(await schicke<Testergebnis>("/api/routing-test", { rolle }));
        setTests([...ergebnisse]);
      }
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    } finally {
      setTestLaeuft(false);
    }
  }

  return (
    <Dialog titel="Einstellungen" offen={offen} schliessen={schliessen} breit>
      {!werte ? (
        <p className="text-sm text-gedaempft">Wird geladen …</p>
      ) : (
        <div className="space-y-6">
          <Feld
            label="OpenRouter-Schlüssel"
            hinweis={
              keyGesetzt
                ? "Ein Schlüssel ist hinterlegt. Er verlässt den Server nie; leer lassen behält den bisherigen."
                : "Ohne Schlüssel kann nichts erzählt werden. Zu finden unter openrouter.ai/keys."
            }
          >
            <input
              type="password"
              className={eingabeKlasse}
              value={key}
              placeholder={keyGesetzt ? "••••••••  (unverändert)" : "sk-or-v1-…"}
              onChange={(e) => setKey(e.target.value)}
            />
          </Feld>

          <div className="space-y-4 rounded-lg border border-rand p-4">
            <h3 className="text-sm tracking-wide text-gedaempft uppercase">Modelle</h3>
            <Feld label="Erzähler" hinweis="Schreibt die Prosa. Darf unzensiert sein.">
              <ModellWahl
                wert={werte.narratorModel}
                setzen={(v) => setzen({ narratorModel: v })}
                modelle={modelle}
              />
            </Feld>
            <Feld
              label="Reserve-Erzähler"
              hinweis="Springt ein, wenn der Erzähler ausfällt oder den Ton verfehlt - und ist die zweite Seite im Modellvergleich."
            >
              <ModellWahl
                wert={werte.reserveModel}
                setzen={(v) => setzen({ reserveModel: v })}
                modelle={modelle}
              />
            </Feld>
            <Feld
              label="Analyst"
              hinweis="Wertet aus statt zu erzählen: braucht striktes JSON und muss denselben Text sehen dürfen. Ab M2 im Einsatz."
            >
              <ModellWahl
                wert={werte.analystModel}
                setzen={(v) => setzen({ analystModel: v })}
                modelle={modelle}
                nurStrictJson
              />
            </Feld>
          </div>

          <div className="space-y-4 rounded-lg border border-rand p-4">
            <h3 className="text-sm tracking-wide text-gedaempft uppercase">Anbieter und Datenschutz</h3>
            {anbieterKonflikt && (
              <div className="rounded-lg border border-amber-800/50 bg-amber-950/20 px-3 py-2 text-xs text-amber-100/90">
                Das Erzählmodell <span className="font-mono">{werte.narratorModel}</span> gehört
                vermutlich nicht zu <span className="font-mono">{werte.narratorProviders.join(", ")}</span>.
                Die Liste gilt für jedes Erzählmodell — passt sie nicht, findet OpenRouter keinen
                Anbieter und der Zug scheitert. Entweder das Feld leeren oder den passenden Anbieter
                eintragen, danach „Alle drei testen".
              </div>
            )}
            <Feld
              label="Erlaubte Anbieter für die Erzählung"
              hinweis="Komma-getrennt. Leer heißt: OpenRouter darf frei wählen. Mit Eintrag gilt die Liste, und Ausweichrouten sind gesperrt."
            >
              <input
                className={eingabeKlasse}
                value={werte.narratorProviders.join(", ")}
                placeholder="venice"
                onChange={(e) =>
                  setzen({
                    narratorProviders: e.target.value
                      .split(",")
                      .map((s) => s.trim())
                      .filter(Boolean),
                  })
                }
              />
            </Feld>
            <label className="flex items-start gap-3 text-sm">
              <input
                type="checkbox"
                className="mt-1"
                checked={werte.narratorZdr}
                onChange={(e) => setzen({ narratorZdr: e.target.checked })}
              />
              <span>
                Zero Data Retention auch für den Erzähler erzwingen
                <span className="mt-0.5 block text-xs text-gedaempft">
                  Vorsicht: Anbieter ohne ZDR-Endpunkt fallen dann aus dem Routing. Wenn dein
                  Erzählmodell nur bei einem Anbieter läuft, scheitert der Aufruf hart.
                </span>
              </span>
            </label>
            <label className="flex items-start gap-3 text-sm">
              <input
                type="checkbox"
                className="mt-1"
                checked={werte.analystZdr}
                onChange={(e) => setzen({ analystZdr: e.target.checked })}
              />
              <span>
                Zero Data Retention für den Analysten erzwingen
                <span className="mt-0.5 block text-xs text-gedaempft">Empfohlen.</span>
              </span>
            </label>
          </div>

          <div className="space-y-4 rounded-lg border border-rand p-4">
            <h3 className="text-sm tracking-wide text-gedaempft uppercase">Autonomie</h3>
            <label className="flex items-start gap-3 text-sm">
              <input
                type="checkbox"
                className="mt-1"
                checked={werte.detektorAn}
                onChange={(e) => setzen({ detektorAn: e.target.checked })}
              />
              <span>
                Jede Antwort auf Gefälligkeit prüfen
                <span className="mt-0.5 block text-xs text-gedaempft">
                  Der Analyst prüft nach dem Streaming gegen sieben Muster — Zustimmung ohne Preis,
                  Spiegeln, unaufgefordertes Lob, bereitwillige Auskunft, weichgespülter Konflikt, keine
                  eigene Initiative, und ob der Text deine Figur spielt. Befunde erscheinen am Zug, der
                  Text bleibt stehen. Kostet rund ein Hundertstel Cent je Zug.
                </span>
              </span>
            </label>
          </div>

          <div className="space-y-4 rounded-lg border border-rand p-4">
            <h3 className="text-sm tracking-wide text-gedaempft uppercase">Gedächtnis</h3>
            <label className="flex items-start gap-3 text-sm">
              <input
                type="checkbox"
                className="mt-1"
                checked={werte.gedaechtnisAn}
                onChange={(e) => setzen({ gedaechtnisAn: e.target.checked })}
              />
              <span>
                Chronik und Fakten führen
                <span className="mt-0.5 block text-xs text-gedaempft">
                  Was aus dem wörtlichen Verlauf fällt, wird zusammengefasst statt vergessen; was
                  dauerhaft gilt, landet im Faktenblatt und kommt bei Bedarf zurück in den Prompt.
                </span>
              </span>
            </label>
            <label className="flex items-start gap-3 text-sm">
              <input
                type="checkbox"
                className="mt-1"
                checked={werte.beziehungenAn}
                onChange={(e) => setzen({ beziehungenAn: e.target.checked })}
              />
              <span>
                Verhältnisse fortschreiben
                <span className="mt-0.5 block text-xs text-gedaempft">
                  Nach jedem Zug beurteilt der Analyst, was sich zwischen den Figuren und dir bewegt
                  hat — höchstens fünf Punkte je Achse und Zug, damit ein einzelner Satz kein
                  Verhältnis umwirft.
                </span>
              </span>
            </label>
            <label className="flex items-start gap-3 text-sm">
              <input
                type="checkbox"
                className="mt-1"
                checked={werte.faktenAutomatisch}
                onChange={(e) => setzen({ faktenAutomatisch: e.target.checked })}
              />
              <span>
                Neue Fakten ohne Rückfrage übernehmen
                <span className="mt-0.5 block text-xs text-gedaempft">
                  Aus heißt: Sie landen als Vorschlag im Gedächtnis und gelten erst, wenn du sie
                  übernimmst. Sauberer, aber du musst regelmäßig durchsehen.
                </span>
              </span>
            </label>
          </div>

          <div className="space-y-4 rounded-lg border border-rand p-4">
            <h3 className="text-sm tracking-wide text-gedaempft uppercase">Kontext</h3>
            <div className="grid gap-4 sm:grid-cols-3">
              <Feld label="Züge im Verlauf" hinweis="Bewusst klein halten.">
                <input
                  type="number"
                  min={2}
                  max={200}
                  className={eingabeKlasse}
                  value={werte.maxHistoryTurns}
                  onChange={(e) => setzen({ maxHistoryTurns: Number(e.target.value) })}
                />
              </Feld>
              <Feld label="Antwortlänge (Token)">
                <input
                  type="number"
                  min={64}
                  max={8192}
                  className={eingabeKlasse}
                  value={werte.maxTokens}
                  onChange={(e) => setzen({ maxTokens: Number(e.target.value) })}
                />
              </Feld>
              <Feld label="Temperatur">
                <input
                  type="number"
                  step={0.05}
                  min={0}
                  max={2}
                  className={eingabeKlasse}
                  value={werte.temperature}
                  onChange={(e) => setzen({ temperature: Number(e.target.value) })}
                />
              </Feld>
            </div>
            <p className="text-xs text-gedaempft">
              Kleine Modelle verlieren Anweisungen in langen Prompts. Mehr Verlauf heißt bei ihnen
              schlechtere Regelbefolgung, nicht bessere - auch wenn das Kontextfenster viel größer ist.
            </p>
          </div>

          <div className="space-y-3 rounded-lg border border-rand p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="text-sm tracking-wide text-gedaempft uppercase">Routing prüfen</h3>
              <Knopf onClick={routingTesten} disabled={testLaeuft || !keyGesetzt}>
                {testLaeuft ? "Prüft …" : "Alle drei testen"}
              </Knopf>
            </div>
            <p className="text-xs text-gedaempft">
              Schickt je einen Aufruf über genau ein Token und zeigt, welcher Anbieter tatsächlich
              bedient - oder warum keiner die Bedingungen erfüllt.
            </p>
            {tests.map((t) => (
              <div
                key={t.rolle}
                className={`rounded-lg border px-3 py-2 text-sm ${
                  t.ok ? "border-emerald-700/50 bg-emerald-900/20" : "border-red-800/60 bg-red-950/30"
                }`}
              >
                <div className="font-medium">
                  {t.rolle === "narrator" ? "Erzähler" : t.rolle === "reserve" ? "Reserve" : "Analyst"}
                  {": "}
                  {t.ok ? `läuft über ${t.anbieter || "unbekannt"}` : "kein Anbieter"}
                </div>
                <div className="text-xs text-gedaempft">{t.modell}</div>
                {t.fehler && <div className="mt-1 text-xs text-red-200">{t.fehler}</div>}
              </div>
            ))}
          </div>

          {zustand && (
            <div className="rounded-lg border border-rand px-3 py-2 text-xs text-gedaempft">
              Version <span className="font-mono text-text/80">{zustand.version}</span>
              {zustand.koennen?.length > 0 && (
                <span> · enthält: {zustand.koennen.join(", ")}</span>
              )}
            </div>
          )}

          <Hinweis text={fehler} />
          {gespeichert && <Hinweis text="Gespeichert." art="gut" />}
          <div className="flex justify-end gap-2">
            <Knopf onClick={schliessen}>Schließen</Knopf>
            <Knopf art="haupt" onClick={speichern}>
              Speichern
            </Knopf>
          </div>
        </div>
      )}
    </Dialog>
  );
}
