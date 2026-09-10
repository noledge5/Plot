import { useEffect, useState } from "react";
import { schicke } from "../api";
import type { Story } from "../types";
import { Dialog, Feld, Hinweis, Knopf, eingabeKlasse } from "./ui";

/**
 * Der System-Prompt gehört dem Nutzer. Die Engine hängt nichts an und schneidet
 * nichts weg - was hier steht, geht genau so an das Modell, gefolgt vom
 * jüngsten Verlauf.
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
  const [fehler, setFehler] = useState("");

  useEffect(() => {
    if (!story) return;
    setTitel(story.title);
    setPrompt(story.systemPrompt);
    setFehler("");
  }, [story]);

  async function speichern() {
    if (!story) return;
    try {
      await schicke(`/api/stories/${story.id}`, { titel, systemPrompt: prompt }, "PUT");
      gespeichert();
      schliessen();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <Dialog titel="System-Prompt" offen={story !== null} schliessen={schliessen} breit>
      <div className="space-y-4">
        <Feld label="Titel">
          <input className={eingabeKlasse} value={titel} onChange={(e) => setTitel(e.target.value)} />
        </Feld>
        <Feld
          label="Anweisung an das Modell"
          hinweis="Geht wörtlich als System-Nachricht raus, gefolgt vom jüngsten Verlauf. Über „was ging raus“ an jedem Zug siehst du den fertigen Anfragekörper."
        >
          <textarea
            className={`${eingabeKlasse} min-h-[24rem] font-mono text-[13px] leading-relaxed`}
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            spellCheck={false}
          />
        </Feld>
        <Hinweis text={fehler} />
        <div className="flex justify-end gap-2">
          <Knopf onClick={schliessen}>Verwerfen</Knopf>
          <Knopf art="haupt" onClick={speichern}>
            Speichern
          </Knopf>
        </div>
      </div>
    </Dialog>
  );
}
