import { useEffect, useState } from "react";
import { hole, schicke } from "../api";
import type { Fakt, Gedaechtnis as Daten } from "../types";
import { Dialog, Hinweis, Knopf, eingabeKlasse } from "./ui";

/**
 * Zeigt, woran sich die Erzählung erinnert: die Chronik als Fließtext und das
 * Faktenblatt als Liste. Beides ist von Hand änderbar — ein Modell, das Fakten
 * gewinnt, liegt zwangsläufig manchmal daneben, und dann muss man eingreifen
 * können, ohne die Geschichte neu zu beginnen.
 */
export default function Gedaechtnis({
  storyId,
  offen,
  schliessen,
}: {
  storyId: number;
  offen: boolean;
  schliessen: () => void;
}) {
  const [daten, setDaten] = useState<Daten | null>(null);
  const [fakten, setFakten] = useState<Fakt[]>([]);
  const [filter, setFilter] = useState<"alle" | "canon" | "proposed" | "retired">("alle");
  const [neu, setNeu] = useState({ betrifft: "", text: "" });
  const [fehler, setFehler] = useState("");

  async function laden() {
    try {
      const [d, f] = await Promise.all([
        hole<Daten>(`/api/stories/${storyId}/chronik`),
        hole<Fakt[]>(`/api/stories/${storyId}/facts`),
      ]);
      setDaten(d);
      setFakten(f);
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    if (offen) laden();
  }, [offen, storyId]);

  if (!offen) return null;

  const imPrompt = new Set((daten?.imPrompt ?? []).map((f) => f.id));
  const sichtbar = fakten.filter((f) => filter === "alle" || f.status === filter);

  async function aendern(f: Fakt, teil: Partial<Fakt>) {
    try {
      await schicke(`/api/facts/${f.id}`, { ...f, ...teil }, "PUT");
      laden();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function stilllegen(f: Fakt) {
    try {
      await schicke(`/api/facts/${f.id}/retire`);
      laden();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function loeschen(f: Fakt) {
    if (!window.confirm(`„${f.text.slice(0, 60)}…“ endgültig löschen?`)) return;
    await schicke(`/api/facts/${f.id}`, undefined, "DELETE");
    laden();
  }

  async function anlegen() {
    if (!neu.text.trim()) return;
    try {
      await schicke(`/api/stories/${storyId}/facts`, { ...neu, gewicht: 70 });
      setNeu({ betrifft: "", text: "" });
      laden();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <Dialog titel="Gedächtnis" offen={offen} schliessen={schliessen} breit>
      <div className="space-y-6">
        <section>
          <h3 className="mb-2 text-sm tracking-wide text-gedaempft uppercase">Was bisher geschah</h3>
          {!daten?.chronik.length ? (
            <p className="text-sm text-gedaempft">
              Noch nichts zusammengefasst. Die Chronik entsteht, sobald Züge aus dem wörtlichen
              Verlauf fallen — was dort herausrutscht, wird festgehalten statt vergessen.
            </p>
          ) : (
            <div className="space-y-2">
              {daten.chronik.map((z) => (
                <div key={z.id} className="flex gap-2 rounded-lg border border-rand bg-grund p-3">
                  <p className="erzaehltext flex-1 text-sm">{z.text}</p>
                  <button
                    className="self-start text-xs text-gedaempft hover:text-red-300"
                    title="Abschnitt verwerfen — die Züge selbst bleiben erhalten"
                    onClick={async () => {
                      await schicke(`/api/summaries/${z.id}`, undefined, "DELETE");
                      laden();
                    }}
                  >
                    ✕
                  </button>
                </div>
              ))}
            </div>
          )}
        </section>

        <section>
          <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-sm tracking-wide text-gedaempft uppercase">Was dauerhaft gilt</h3>
            <div className="flex gap-1 text-xs">
              {(["alle", "canon", "proposed", "retired"] as const).map((f) => (
                <button
                  key={f}
                  onClick={() => setFilter(f)}
                  className={`rounded px-2 py-1 ${
                    filter === f ? "bg-flaeche text-text" : "text-gedaempft hover:text-text"
                  }`}
                >
                  {f === "alle"
                    ? "alle"
                    : f === "canon"
                      ? "gültig"
                      : f === "proposed"
                        ? "Vorschläge"
                        : "stillgelegt"}
                </button>
              ))}
            </div>
          </div>

          <div className="mb-3 flex flex-wrap gap-2">
            <input
              className={`${eingabeKlasse} sm:w-40`}
              placeholder="betrifft (Name)"
              value={neu.betrifft}
              onChange={(e) => setNeu({ ...neu, betrifft: e.target.value })}
            />
            <input
              className={`${eingabeKlasse} flex-1`}
              placeholder="Mira schuldet dem Brauer seit dem Frühjahr Geld."
              value={neu.text}
              onChange={(e) => setNeu({ ...neu, text: e.target.value })}
              onKeyDown={(e) => e.key === "Enter" && anlegen()}
            />
            <Knopf onClick={anlegen}>Eintragen</Knopf>
          </div>

          {sichtbar.length === 0 && (
            <p className="text-sm text-gedaempft">Hier steht noch nichts.</p>
          )}

          <ul className="space-y-2">
            {sichtbar.map((f) => (
              <li
                key={f.id}
                className={`rounded-lg border p-3 ${
                  f.status === "retired"
                    ? "border-rand/50 bg-grund/40 opacity-60"
                    : imPrompt.has(f.id)
                      ? "border-akzent/40 bg-akzent/5"
                      : "border-rand bg-grund"
                }`}
              >
                <div className="flex items-start gap-2">
                  <div className="min-w-0 flex-1">
                    <input
                      className="w-full bg-transparent text-sm outline-none"
                      value={f.text}
                      onChange={(e) =>
                        setFakten((alle) =>
                          alle.map((a) => (a.id === f.id ? { ...a, text: e.target.value } : a)),
                        )
                      }
                      onBlur={() => aendern(f, { text: f.text })}
                    />
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-gedaempft">
                      {f.betrifft && <span>{f.betrifft}</span>}
                      <span>Gewicht {f.gewicht}</span>
                      {f.status === "proposed" && <span className="text-akzent">Vorschlag</span>}
                      {f.status === "retired" && <span>stillgelegt</span>}
                      {imPrompt.has(f.id) && <span className="text-akzent/80">gerade im Prompt</span>}
                    </div>
                  </div>
                  <div className="flex shrink-0 flex-wrap gap-1">
                    {f.status === "proposed" && (
                      <button
                        className="rounded border border-rand px-2 py-0.5 text-xs hover:border-akzent"
                        onClick={() => aendern(f, { status: "canon" })}
                      >
                        übernehmen
                      </button>
                    )}
                    <button
                      className={`rounded border px-2 py-0.5 text-xs ${
                        f.angeheftet ? "border-akzent/60 text-akzent" : "border-rand hover:border-gedaempft"
                      }`}
                      title="Angeheftete Fakten stehen immer im Prompt"
                      onClick={() => aendern(f, { angeheftet: !f.angeheftet })}
                    >
                      {f.angeheftet ? "angeheftet" : "anheften"}
                    </button>
                    {f.status !== "retired" && (
                      <button
                        className="rounded border border-rand px-2 py-0.5 text-xs hover:border-gedaempft"
                        title="Gilt ab jetzt nicht mehr — bleibt aber lesbar"
                        onClick={() => stilllegen(f)}
                      >
                        gilt nicht mehr
                      </button>
                    )}
                    <button
                      className="rounded border border-rand px-2 py-0.5 text-xs text-red-300 hover:border-red-500/60"
                      onClick={() => loeschen(f)}
                    >
                      ✕
                    </button>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        </section>

        <Hinweis text={fehler} />
      </div>
    </Dialog>
  );
}
