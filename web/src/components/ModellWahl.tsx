import { useMemo, useState } from "react";
import type { Modell } from "../types";
import { eingabeKlasse } from "./ui";

/**
 * Auswahl aus dem Modellkatalog. Der Katalog hat mehrere hundert Einträge,
 * deshalb Suchfeld statt Aufklappliste - und die Preise stehen dabei, weil sie
 * die eigentliche Entscheidungsgrundlage sind.
 */
export default function ModellWahl({
  wert,
  setzen,
  modelle,
  nurStrictJson,
}: {
  wert: string;
  setzen: (id: string) => void;
  modelle: Modell[];
  nurStrictJson?: boolean;
}) {
  const [suche, setSuche] = useState("");
  const [offen, setOffen] = useState(false);

  const treffer = useMemo(() => {
    const s = suche.trim().toLowerCase();
    return modelle
      .filter((m) => !nurStrictJson || m.strictJson)
      .filter((m) => !s || m.id.toLowerCase().includes(s) || m.name.toLowerCase().includes(s))
      .slice(0, 40);
  }, [suche, modelle, nurStrictJson]);

  const gewaehlt = modelle.find((m) => m.id === wert);

  return (
    <div className="relative">
      <input
        className={eingabeKlasse}
        value={offen ? suche : wert}
        placeholder="Modellkennung, z. B. mistralai/mistral-nemo"
        onFocus={() => {
          setOffen(true);
          setSuche("");
        }}
        onChange={(e) => {
          setSuche(e.target.value);
          setzen(e.target.value);
        }}
        onBlur={() => setTimeout(() => setOffen(false), 150)}
      />
      {gewaehlt && !offen && (
        <p className="mt-1 text-xs text-gedaempft">
          {gewaehlt.promptPerM.toFixed(3)} $ / {gewaehlt.outputPerM.toFixed(3)} $ je Mio. ·{" "}
          {(gewaehlt.contextLength / 1000).toFixed(0)}k Kontext ·{" "}
          {gewaehlt.strictJson ? "striktes JSON" : "kein striktes JSON"}
        </p>
      )}
      {offen && treffer.length > 0 && (
        <ul className="absolute z-20 mt-1 max-h-72 w-full overflow-y-auto rounded-lg border border-rand bg-flaeche shadow-xl">
          {treffer.map((m) => (
            <li key={m.id}>
              <button
                type="button"
                className="block w-full px-3 py-2 text-left hover:bg-grund"
                onMouseDown={() => {
                  setzen(m.id);
                  setOffen(false);
                }}
              >
                <div className="truncate text-sm">{m.id}</div>
                <div className="text-xs text-gedaempft">
                  {m.promptPerM.toFixed(3)} / {m.outputPerM.toFixed(3)} $ je Mio. ·{" "}
                  {(m.contextLength / 1000).toFixed(0)}k
                  {m.strictJson ? " · striktes JSON" : ""}
                </div>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
