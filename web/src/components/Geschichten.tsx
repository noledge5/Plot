import { useEffect, useState } from "react";
import { hole, schicke } from "../api";
import type { Story } from "../types";
import { Hinweis, Knopf } from "./ui";

export default function Geschichten({
  oeffnen,
  einstellungenOeffnen,
  keyGesetzt,
  abmelden,
}: {
  oeffnen: (id: number) => void;
  einstellungenOeffnen: () => void;
  keyGesetzt: boolean;
  abmelden: () => void;
}) {
  const [stories, setStories] = useState<Story[]>([]);
  const [fehler, setFehler] = useState("");

  async function laden() {
    try {
      setStories(await hole<Story[]>("/api/stories"));
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    laden();
  }, []);

  async function neu() {
    try {
      const s = await schicke<Story>("/api/stories", { titel: "Neue Geschichte" });
      oeffnen(s.id);
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function loeschen(s: Story) {
    if (!window.confirm(`„${s.title}“ mit allen Zügen und Speicherständen löschen?`)) return;
    try {
      await schicke(`/api/stories/${s.id}`, undefined, "DELETE");
      laden();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="mx-auto max-w-2xl px-4 py-8">
      <header className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl tracking-tight">Plot</h1>
        <div className="flex gap-2">
          <Knopf onClick={einstellungenOeffnen}>Einstellungen</Knopf>
          <Knopf onClick={abmelden} titel="Abmelden">
            ⏻
          </Knopf>
        </div>
      </header>

      {!keyGesetzt && (
        <div className="mb-6 rounded-lg border border-akzent/40 bg-akzent/10 p-4 text-sm">
          Es ist noch kein OpenRouter-Schlüssel hinterlegt. Ohne ihn kann nichts erzählt werden.
          <Knopf klasse="mt-3" art="haupt" onClick={einstellungenOeffnen}>
            Jetzt eintragen
          </Knopf>
        </div>
      )}

      <Hinweis text={fehler} />

      <div className="mt-4 space-y-2">
        {stories.map((s) => (
          <div
            key={s.id}
            className="flex items-center gap-2 rounded-lg border border-rand bg-flaeche p-3 hover:border-gedaempft"
          >
            <button className="min-w-0 flex-1 text-left" onClick={() => oeffnen(s.id)}>
              <div className="truncate">{s.title}</div>
              <div className="text-xs text-gedaempft">
                zuletzt {new Date(s.updatedAt).toLocaleString("de-DE")}
              </div>
            </button>
            <Knopf art="gefahr" onClick={() => loeschen(s)} titel="Löschen">
              ✕
            </Knopf>
          </div>
        ))}
      </div>

      <Knopf klasse="mt-6 w-full" art="haupt" onClick={neu}>
        Neue Geschichte
      </Knopf>
    </div>
  );
}
