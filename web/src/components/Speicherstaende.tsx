import { useEffect, useState } from "react";
import { hole, schicke } from "../api";
import type { Slot } from "../types";
import { Hinweis, Knopf, eingabeKlasse } from "./ui";

/**
 * Speicherstände sind Zeiger auf einen Knoten im Baum. Laden setzt nur den
 * Zeiger - es wird nichts kopiert und nichts gelöscht, der verlassene Zweig
 * bleibt vollständig erhalten.
 */
export default function Speicherstaende({
  storyId,
  geladen,
  aktualisierung,
}: {
  storyId: number;
  geladen: () => void;
  aktualisierung: number;
}) {
  const [slots, setSlots] = useState<Slot[]>([]);
  const [name, setName] = useState("");
  const [fehler, setFehler] = useState("");

  async function laden() {
    try {
      setSlots(await hole<Slot[]>(`/api/stories/${storyId}/slots`));
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    laden();
  }, [storyId, aktualisierung]);

  async function anlegen() {
    setFehler("");
    try {
      await schicke(`/api/stories/${storyId}/slots`, { name: name.trim() || "Speicherstand" });
      setName("");
      laden();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function hierher(slot: Slot) {
    try {
      await schicke(`/api/slots/${slot.id}/load`);
      geladen();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function loeschen(slot: Slot) {
    try {
      await schicke(`/api/slots/${slot.id}`, undefined, "DELETE");
      laden();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <input
          className={eingabeKlasse}
          placeholder="Name des Speicherstands"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && anlegen()}
        />
        <Knopf onClick={anlegen}>Sichern</Knopf>
      </div>
      <Hinweis text={fehler} />
      {slots.length === 0 && (
        <p className="text-sm text-gedaempft">
          Noch nichts gesichert. Automatische Stände entstehen alle fünf Züge von selbst.
        </p>
      )}
      <ul className="space-y-2">
        {slots.map((s) => (
          <li key={s.id} className="rounded-lg border border-rand bg-grund p-3">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm">{s.name}</span>
                  {s.kind === "auto" && (
                    <span className="rounded bg-flaeche px-1.5 py-0.5 text-[10px] text-gedaempft">auto</span>
                  )}
                </div>
                {s.preview && <p className="mt-1 line-clamp-2 text-xs text-gedaempft">{s.preview}</p>}
              </div>
              <div className="flex shrink-0 gap-1">
                <Knopf onClick={() => hierher(s)} titel="Hierher zurückkehren">
                  Laden
                </Knopf>
                <Knopf art="gefahr" onClick={() => loeschen(s)} titel="Speicherstand entfernen">
                  ✕
                </Knopf>
              </div>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
