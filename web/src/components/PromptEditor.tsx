import { useEffect, useState } from "react";
import { hole, schicke } from "../api";
import type { PromptBlock, Story, StorySettings } from "../types";
import { Dialog, Feld, Hinweis, Knopf, eingabeKlasse } from "./ui";

type Vorlage = { systemPrompt: string; platzhalter: { name: string; was: string }[] };

// Der Absatz, den die aktuelle Vorlage zur Erzählzeit führt.
const PRAESENS_ABSATZ = `Erzählzeit Präsens: Du erzählst, was gerade geschieht, nicht, was geschehen
ist. Die Figur des Spielers sprichst du mit "du" an, alle anderen Figuren in
der dritten Person.`;

/**
 * Ersetzt nur die Aussage zur Erzählzeit und lässt alles andere stehen.
 *
 * Die ganze Vorlage zu übernehmen wirft weg, was der Nutzer selbst
 * hineingeschrieben hat. Wer nur von Vergangenheit auf Gegenwart will, soll
 * nicht seinen Prompt verlieren — deshalb dieser chirurgische Schnitt.
 */
function aufPraesens(prompt: string): string {
  // Der Satz aus der alten Vorlage, wortgleich.
  const alt = /Erzählzeit Präteritum, dritte Person\.?/;
  if (alt.test(prompt)) return prompt.replace(alt, PRAESENS_ABSATZ);
  // Freier formulierte Prompts: nur das Wort tauschen und die Klarstellung
  // dahinter setzen, damit „dritte Person" nicht gegen „du" steht.
  if (/Präteritum|Vergangenheit|Imperfekt/i.test(prompt)) {
    return prompt.replace(/Präteritum|Vergangenheit|Imperfekt/gi, "Präsens");
  }
  return prompt;
}

/** Sagt, ob im Prompt überhaupt etwas auf Vergangenheit hindeutet. */
function klingtNachVergangenheit(prompt: string): boolean {
  return /Präteritum|Vergangenheit|Imperfekt/i.test(prompt);
}

const leereSettings: StorySettings = {
  stilbeispiel: "",
  szene: "",
  welt: "",
  autorNotiz: "",
  druckSchwelle: 60,
  erzaehlzeit: "praesens",
};

const ZEITEN: { wert: string; name: string }[] = [
  { wert: "praesens", name: "Präsens" },
  { wert: "praeteritum", name: "Präteritum" },
  { wert: "aus", name: "nicht vorgeben" },
];

/**
 * Der System-Prompt gehört dem Nutzer. Die Engine setzt nur ein, was er als
 * Platzhalter hingeschrieben hat — und die Vorschau zeigt vorher, was dabei
 * herauskommt.
 */
export default function PromptEditor({
  story,
  schliessen,
  gespeichert,
}: {
  story: Story | null;
  schliessen: () => void;
  gespeichert: () => void;
}) {
  const [titel, setTitel] = useState("");
  const [prompt, setPrompt] = useState("");
  const [settings, setSettings] = useState<StorySettings>(leereSettings);
  const [fehler, setFehler] = useState("");
  const [vorschau, setVorschau] = useState<{ text: string; tokens: number; bloecke: PromptBlock[] } | null>(
    null,
  );
  // Die Engine kennt mehr Blöcke, als in einem alten Prompt stehen können:
  // Eine Geschichte behält ihren Prompt für immer, sonst überschriebe ein
  // Update den Text des Nutzers. Also holen wir die aktuelle Vorlage und
  // sagen, welcher Block fehlt.
  const [vorlage, setVorlage] = useState<Vorlage | null>(null);

  useEffect(() => {
    hole<Vorlage>("/api/vorlage")
      .then(setVorlage)
      .catch(() => setVorlage(null));
  }, []);

  useEffect(() => {
    if (!story) return;
    setTitel(story.title);
    setPrompt(story.systemPrompt);
    setSettings({ ...leereSettings, ...(story.settings ?? {}) });
    setFehler("");
    setVorschau(null);
  }, [story]);

  async function speichern() {
    if (!story) return;
    try {
      await schicke(`/api/stories/${story.id}`, { titel, systemPrompt: prompt, settings }, "PUT");
      gespeichert();
      schliessen();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function zeigeVorschau() {
    if (!story) return;
    setFehler("");
    try {
      // Erst speichern, damit Stilbeispiel und Szene in die Vorschau eingehen.
      await schicke(`/api/stories/${story.id}`, { titel, systemPrompt: prompt, settings }, "PUT");
      setVorschau(await schicke(`/api/stories/${story.id}/preview`, { systemPrompt: prompt }));
      gespeichert();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  const setzen = (teil: Partial<StorySettings>) => setSettings((s) => ({ ...s, ...teil }));

  const platzhalter = vorlage?.platzhalter ?? [];
  const fehlend = platzhalter.filter((p) => !prompt.includes(`{{${p.name}}}`));

  function erzaehlzeitUmstellen() {
    setPrompt((t) => aufPraesens(t));
    setVorschau(null);
  }

  function vorlageUebernehmen() {
    if (!vorlage) return;
    const warnung =
      "Deine Anweisung wird durch die aktuelle Vorlage ersetzt. " +
      "Was du selbst hineingeschrieben hast, ist danach weg. Fortfahren?";
    if (!window.confirm(warnung)) return;
    setPrompt(vorlage.systemPrompt);
    setVorschau(null);
  }

  return (
    <Dialog titel="System-Prompt" offen={story !== null} schliessen={schliessen} breit>
      <div className="space-y-5">
        <Feld label="Titel">
          <input className={eingabeKlasse} value={titel} onChange={(e) => setTitel(e.target.value)} />
        </Feld>

        <Feld
          label="Anweisung an das Modell"
          hinweis="Geht wörtlich raus, nachdem die Platzhalter gefüllt sind. Unbekannte Platzhalter bleiben stehen — so fällt ein Tippfehler auf."
        >
          <textarea
            className={`${eingabeKlasse} min-h-[22rem] font-mono text-[13px] leading-relaxed`}
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            spellCheck={false}
          />
        </Feld>

        {klingtNachVergangenheit(prompt) && (
          <div className="rounded-lg border border-amber-800/40 bg-amber-950/20 p-3 text-sm">
            <p className="text-amber-100/90">
              Deine Anweisung verlangt Vergangenheit. Solange sie das tut, erzählt das Modell in der
              Vergangenheit — die Engine schreibt nichts in deinen Prompt hinein.
            </p>
            <Knopf klasse="mt-2" onClick={erzaehlzeitUmstellen}>
              Erzählzeit auf Präsens
            </Knopf>
            <span className="ml-2 text-xs text-amber-100/50">
              ändert nur diesen Satz, sonst nichts
            </span>
          </div>
        )}

        {fehlend.length > 0 && (
          <div className="rounded-lg border border-amber-800/40 bg-amber-950/20 p-3 text-sm">
            <p className="text-amber-100/90">
              Diese Blöcke füllt die Engine, aber sie stehen nicht in deiner Anweisung — ihr Inhalt
              geht nicht an das Modell:{" "}
              <span className="font-mono text-xs">
                {fehlend.map((p) => `{{${p.name}}}`).join(", ")}
              </span>
            </p>
            <p className="mt-1 text-xs text-amber-100/60">
              Einzeln unten anhängen — oder unten die aktuelle Vorlage übernehmen.
            </p>
          </div>
        )}

        <div className="rounded-lg border border-rand p-3">
          <h3 className="mb-2 text-sm tracking-wide text-gedaempft uppercase">Platzhalter</h3>
          <div className="flex flex-wrap gap-2">
            {platzhalter.map((p) => {
              const drin = prompt.includes(`{{${p.name}}}`);
              return (
                <button
                  key={p.name}
                  title={p.was + (drin ? " — bereits im Prompt" : " — anhängen")}
                  onClick={() => !drin && setPrompt((t) => t.trimEnd() + `\n\n{{${p.name}}}`)}
                  className={`rounded border px-2 py-1 font-mono text-xs ${
                    drin
                      ? "border-akzent/50 bg-akzent/10 text-akzent"
                      : "border-rand text-gedaempft hover:border-gedaempft"
                  }`}
                >
                  {`{{${p.name}}}`}
                </button>
              );
            })}
          </div>
          {vorlage && (
            <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-rand pt-3">
              <p className="text-xs text-gedaempft">
                Deine Geschichte behält ihre Anweisung für immer — ein Update überschreibt nie, was
                du geschrieben hast. Wenn du den aktuellen Stand der Engine willst, hol ihn dir
                hier.
              </p>
              <Knopf onClick={vorlageUebernehmen}>Vorlage übernehmen</Knopf>
            </div>
          )}
        </div>

        <Feld
          label="Stilbeispiel"
          hinweis="Ein, zwei Absätze in dem Ton, den du willst. Kleine Modelle imitieren Stil deutlich besser, als sie Stilanweisungen befolgen — das ist hier der größte Hebel. Wird als Tonprobe gekennzeichnet, nicht als Handlung."
        >
          <textarea
            className={`${eingabeKlasse} min-h-32 leading-relaxed`}
            value={settings.stilbeispiel}
            placeholder="Der Regen hört auf, als sie die Tür hinter sich schließt. Sie bleibt im Flur stehen, den Mantel noch an, und horcht …"
            onChange={(e) => setzen({ stilbeispiel: e.target.value })}
          />
        </Feld>

        <div className="grid gap-4 sm:grid-cols-2">
          <Feld label="Welt" hinweis="Statischer Hintergrund.">
            <textarea
              className={`${eingabeKlasse} min-h-24`}
              value={settings.welt}
              onChange={(e) => setzen({ welt: e.target.value })}
            />
          </Feld>
          <Feld label="Szene" hinweis="Ort, Zeit, Lage — was gerade gilt.">
            <textarea
              className={`${eingabeKlasse} min-h-24`}
              value={settings.szene}
              onChange={(e) => setzen({ szene: e.target.value })}
            />
          </Feld>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <Feld label="Regieanweisung" hinweis="Gilt für den nächsten Zug.">
            <input
              className={eingabeKlasse}
              value={settings.autorNotiz}
              placeholder="Lass die Szene enden, ohne dass etwas geklärt wird."
              onChange={(e) => setzen({ autorNotiz: e.target.value })}
            />
          </Feld>
          <Feld
            label="Erzählzeit"
            hinweis="Geht bei jedem Zug als Direktive mit. Nötig, weil ein Modell fortsetzt, was im Verlauf schon dasteht — ein Satz weit oben im Prompt verliert dagegen."
          >
            <div className="flex gap-1 text-sm">
              {ZEITEN.map((z) => (
                <button
                  key={z.wert}
                  onClick={() => setzen({ erzaehlzeit: z.wert })}
                  className={`rounded px-2 py-1 transition-colors ${
                    (settings.erzaehlzeit || "praesens") === z.wert
                      ? "bg-akzent/15 text-akzent"
                      : "text-gedaempft hover:text-text"
                  }`}
                >
                  {z.name}
                </button>
              ))}
            </div>
          </Feld>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <Feld
            label={`Druckschwelle: ${settings.druckSchwelle}`}
            hinweis="Ab hier verfolgt eine Figur ihr Ziel aktiv, auch wenn es unpassend ist."
          >
            <input
              type="range"
              min={10}
              max={100}
              step={5}
              className="w-full"
              value={settings.druckSchwelle}
              onChange={(e) => setzen({ druckSchwelle: Number(e.target.value) })}
            />
          </Feld>
        </div>

        <Hinweis text={fehler} />

        {vorschau && (
          <div className="space-y-2 rounded-lg border border-rand bg-grund p-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="text-sm tracking-wide text-gedaempft uppercase">So geht er raus</h3>
              <span className="text-xs text-gedaempft">~{vorschau.tokens} Tokens</span>
            </div>
            <div className="flex flex-wrap gap-1">
              {vorschau.bloecke.map((b) => (
                <span
                  key={b.platzhalter}
                  className={`rounded px-1.5 py-0.5 font-mono text-[10px] ${
                    b.tokens > 1 ? "bg-flaeche text-gedaempft" : "bg-flaeche/50 text-gedaempft/50"
                  }`}
                  title={b.tokens > 1 ? `${b.tokens} Tokens` : "leer"}
                >
                  {b.platzhalter} {b.tokens > 1 ? `· ${b.tokens}` : "· leer"}
                </span>
              ))}
            </div>
            <pre className="max-h-80 overflow-y-auto text-xs whitespace-pre-wrap text-text/85">
              {vorschau.text}
            </pre>
          </div>
        )}

        <div className="flex flex-wrap justify-end gap-2">
          <Knopf onClick={schliessen}>Schließen</Knopf>
          <Knopf onClick={zeigeVorschau}>Vorschau</Knopf>
          <Knopf art="haupt" onClick={speichern}>
            Speichern
          </Knopf>
        </div>
      </div>
    </Dialog>
  );
}
