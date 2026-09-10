import { useEffect, useState } from "react";
import { hole } from "../api";
import type { Inspektion, Protokoll } from "../types";
import { Dialog, Hinweis, geld } from "./ui";

/**
 * Zeigt den exakten Anfragekörper, der an OpenRouter ging. Ohne ihn lässt sich
 * nie klären, ob eine schwache Antwort am Modell oder am Prompt lag.
 */
export default function Inspektor({
  nodeId,
  schliessen,
}: {
  nodeId: number | null;
  schliessen: () => void;
}) {
  const [protokoll, setProtokoll] = useState<Protokoll | null>(null);
  const [weitere, setWeitere] = useState<Protokoll[]>([]);
  const [fehler, setFehler] = useState("");

  useEffect(() => {
    if (nodeId === null) return;
    setProtokoll(null);
    setWeitere([]);
    setFehler("");
    hole<Inspektion>(`/api/nodes/${nodeId}/inspect`)
      .then((d) => {
        setProtokoll(d.erzaehlung);
        setWeitere((d.alle ?? []).filter((r) => r.purpose !== "narrate"));
      })
      .catch((e) => setFehler(e instanceof Error ? e.message : String(e)));
  }, [nodeId]);

  let hübsch = protokoll?.request ?? "";
  let nachrichten: { role: string; content: string }[] = [];
  if (protokoll) {
    try {
      const daten = JSON.parse(protokoll.request);
      nachrichten = daten.messages ?? [];
      hübsch = JSON.stringify(daten, null, 2);
    } catch {
      /* Rohtext anzeigen */
    }
  }

  return (
    <Dialog titel="Was tatsächlich verschickt wurde" offen={nodeId !== null} schliessen={schliessen} breit>
      <Hinweis text={fehler} />
      {protokoll && (
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
            <Kennzahl name="Modell" wert={protokoll.model} klein />
            <Kennzahl name="Kosten" wert={geld(protokoll.costUsd)} />
            <Kennzahl name="Dauer" wert={`${(protokoll.latencyMs / 1000).toFixed(1)} s`} />
            <Kennzahl name="Blöcke" wert={String(nachrichten.length)} />
          </div>
          {protokoll.error && <Hinweis text={protokoll.error} />}

          {nachrichten.length > 0 && (
            <div className="space-y-2">
              <h3 className="text-sm tracking-wide text-gedaempft uppercase">Nachrichtenfolge</h3>
              {nachrichten.map((m, i) => (
                <details key={i} className="rounded-lg border border-rand bg-grund" open={m.role === "system"}>
                  <summary className="cursor-pointer px-3 py-2 text-xs text-gedaempft">
                    {m.role} · {m.content.length} Zeichen
                  </summary>
                  <pre className="overflow-x-auto px-3 pb-3 text-xs whitespace-pre-wrap text-text/90">
                    {m.content}
                  </pre>
                </details>
              ))}
            </div>
          )}

          <details className="rounded-lg border border-rand bg-grund">
            <summary className="cursor-pointer px-3 py-2 text-xs text-gedaempft">
              Vollständiger Anfragekörper
            </summary>
            <pre className="overflow-x-auto px-3 pb-3 text-xs whitespace-pre-wrap text-text/80">{hübsch}</pre>
          </details>

          {weitere.length > 0 && (
            <div className="space-y-2">
              <h3 className="text-sm tracking-wide text-gedaempft uppercase">Weitere Aufrufe zu diesem Zug</h3>
              {weitere.map((r) => (
                <details key={r.id} className="rounded-lg border border-rand bg-grund">
                  <summary className="cursor-pointer px-3 py-2 text-xs text-gedaempft">
                    {r.purpose} · {r.model} · {geld(r.costUsd)} · {(r.latencyMs / 1000).toFixed(1)} s
                    {r.error && " · fehlgeschlagen"}
                  </summary>
                  <pre className="overflow-x-auto px-3 pb-3 text-xs whitespace-pre-wrap text-text/70">
                    {r.error || r.response}
                  </pre>
                </details>
              ))}
            </div>
          )}
        </div>
      )}
    </Dialog>
  );
}

function Kennzahl({ name, wert, klein }: { name: string; wert: string; klein?: boolean }) {
  return (
    <div className="rounded-lg border border-rand bg-grund px-3 py-2">
      <div className="text-xs text-gedaempft">{name}</div>
      <div className={klein ? "truncate text-xs" : "text-sm"} title={wert}>
        {wert}
      </div>
    </div>
  );
}
