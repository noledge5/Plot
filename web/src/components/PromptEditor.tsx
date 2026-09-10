import { useEffect, useState } from "react";
import { schicke } from "../api";
import type { PromptBlock, Story, StorySettings } from "../types";
import { Dialog, Feld, Hinweis, Knopf, eingabeKlasse } from "./ui";

const PLATZHALTER: { name: string; was: string }[] = [
  { name: "persona", was: "Blatt deiner Figur" },
  { name: "characters", was: "Blätter der anwesenden Figuren" },
  { name: "limits", was: "Grenzen und Geheimnisse, als Liste" },
  { name: "drives", was: "was die Figuren wollen" },
  { name: "stilbeispiel", was: "deine Tonprobe" },
  { name: "welt", was: "Hintergrund" },
  { name: "szene", was: "Ort, Zeit, Lage" },
  { name: "directives", was: "Regieanweisung der Engine" },
  { name: "autornotiz", was: "deine Anweisung für den nächsten Zug" },
];

const leereSettings: StorySettings = {
  stilbeispiel: "",
  szene: "",
  welt: "",
  autorNotiz: "",
  druckSchwelle: 60,
};

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

        <div className="rounded-lg border border-rand p-3">
          <h3 className="mb-2 text-sm tracking-wide text-gedaempft uppercase">Platzhalter</h3>
          <div className="flex flex-wrap gap-2">
            {PLATZHALTER.map((p) => {
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
        </div>

        <Feld
          label="Stilbeispiel"
          hinweis="Ein, zwei Absätze in dem Ton, den du willst. Kleine Modelle imitieren Stil deutlich besser, als sie Stilanweisungen befolgen — das ist hier der größte Hebel. Wird als Tonprobe gekennzeichnet, nicht als Handlung."
        >
          <textarea
            className={`${eingabeKlasse} min-h-32 leading-relaxed`}
            value={settings.stilbeispiel}
            placeholder="Der Regen hörte auf, als sie die Tür hinter sich schloss. Sie blieb im Flur stehen, den Mantel noch an, und horchte …"
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
