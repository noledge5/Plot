import { useEffect, useRef, useState } from "react";
import { hole, schicke, strom } from "../api";
import type { Einstellungen, Knoten, Story, Verbrauch } from "../types";
import Inspektor from "./Inspektor";
import Speicherstaende from "./Speicherstaende";
import { Hinweis, Knopf, eingabeKlasse, geld } from "./ui";

type PfadAntwort = { story: Story; knoten: Knoten[]; kosten: number; aufrufe: number };

type Vergleich = {
  aktiv: boolean;
  modellA: string;
  modellB: string;
  textA: string;
  textB: string;
  nodeA: number | null;
  nodeB: number | null;
  fertig: boolean;
};

const leererVergleich: Vergleich = {
  aktiv: false,
  modellA: "",
  modellB: "",
  textA: "",
  textB: "",
  nodeA: null,
  nodeB: null,
  fertig: false,
};

export default function Spiel({
  storyId,
  zurueck,
  promptBearbeiten,
}: {
  storyId: number;
  zurueck: () => void;
  promptBearbeiten: (s: Story) => void;
}) {
  const [story, setStory] = useState<Story | null>(null);
  const [knoten, setKnoten] = useState<Knoten[]>([]);
  const [kosten, setKosten] = useState(0);
  const [eingabe, setEingabe] = useState("");
  const [laufend, setLaufend] = useState(false);
  const [teilText, setTeilText] = useState("");
  const [fehler, setFehler] = useState("");
  const [inspektor, setInspektor] = useState<number | null>(null);
  const [seitenleiste, setSeitenleiste] = useState(false);
  const [slotStand, setSlotStand] = useState(0);
  const [vergleich, setVergleich] = useState<Vergleich>(leererVergleich);
  const [letzterZug, setLetzterZug] = useState<{ modell: string; anbieter: string; usage: Verbrauch } | null>(null);
  const [reserveModell, setReserveModell] = useState("");

  const abbruch = useRef<AbortController | null>(null);
  const ende = useRef<HTMLDivElement>(null);

  async function ladePfad() {
    try {
      const d = await hole<PfadAntwort>(`/api/stories/${storyId}/path`);
      setStory(d.story);
      setKnoten(d.knoten);
      setKosten(d.kosten);
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    ladePfad();
  }, [storyId]);

  // Für "mit dem Reservemodell wiederholen" - der Knopf muss wissen, welches
  // Modell das ist, sonst wiederholt er nur dasselbe.
  useEffect(() => {
    hole<{ settings: Einstellungen }>("/api/settings")
      .then((d) => setReserveModell(d.settings.reserveModel))
      .catch(() => setReserveModell(""));
  }, []);

  useEffect(() => {
    ende.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [knoten.length, teilText, vergleich.textA, vergleich.textB]);

  const letzterKnoten = knoten.length ? knoten[knoten.length - 1] : null;

  async function senden() {
    const text = eingabe.trim();
    if (!text && knoten.length === 0) return;
    setFehler("");
    setTeilText("");
    setLaufend(true);
    setEingabe("");
    abbruch.current = new AbortController();
    try {
      await strom(
        `/api/stories/${storyId}/turn`,
        { text },
        (ereignis, daten) => {
          if (ereignis === "delta") setTeilText((t) => t + daten.text);
          else if (ereignis === "fehler") setFehler(daten.fehler);
          else if (ereignis === "fertig") {
            setLetzterZug({ modell: daten.modell, anbieter: daten.anbieter, usage: daten.usage });
          }
        },
        abbruch.current.signal,
      );
    } catch (e) {
      // Ein Abbruch durch den Nutzer ist kein Fehler: der Text bis dahin ist
      // bereits gespeichert.
      if (!(e instanceof DOMException && e.name === "AbortError")) {
        setFehler(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setLaufend(false);
      setTeilText("");
      abbruch.current = null;
      await ladePfad();
      setSlotStand((n) => n + 1);
    }
  }

  async function wiederholen(nodeId: number, modell?: string) {
    setFehler("");
    setTeilText("");
    setLaufend(true);
    abbruch.current = new AbortController();
    try {
      await strom(
        `/api/stories/${storyId}/regenerate`,
        { nodeId, modell },
        (ereignis, daten) => {
          if (ereignis === "delta") setTeilText((t) => t + daten.text);
          else if (ereignis === "fehler") setFehler(daten.fehler);
          else if (ereignis === "fertig")
            setLetzterZug({ modell: daten.modell, anbieter: daten.anbieter, usage: daten.usage });
        },
        abbruch.current.signal,
      );
    } catch (e) {
      if (!(e instanceof DOMException && e.name === "AbortError")) {
        setFehler(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setLaufend(false);
      setTeilText("");
      abbruch.current = null;
      await ladePfad();
    }
  }

  /** Blättert zwischen den Fassungen desselben Zuges. */
  async function fassungWechseln(nodeId: number, richtung: 1 | -1) {
    try {
      const alt = await hole<Knoten[]>(`/api/nodes/${nodeId}/alternatives`);
      if (alt.length < 2) return;
      const i = alt.findIndex((a) => a.id === nodeId);
      const ziel = alt[(i + richtung + alt.length) % alt.length];
      await schicke(`/api/nodes/${ziel.id}/head`);
      await ladePfad();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function textAendern(n: Knoten) {
    const neu = window.prompt("Text ändern", n.content);
    if (neu === null || neu === n.content) return;
    try {
      await schicke(`/api/nodes/${n.id}`, { inhalt: neu }, "PUT");
      await ladePfad();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function vergleichStarten() {
    const text = eingabe.trim();
    setFehler("");
    setEingabe("");
    setLaufend(true);
    setVergleich({ ...leererVergleich, aktiv: true, modellA: "", modellB: "" });
    abbruch.current = new AbortController();
    try {
      await strom(
        `/api/stories/${storyId}/compare`,
        { text },
        (ereignis, daten) => {
          if (ereignis === "delta") {
            setVergleich((v) =>
              daten.seite === "a" ? { ...v, textA: v.textA + daten.text } : { ...v, textB: v.textB + daten.text },
            );
          } else if (ereignis === "seiteFertig") {
            setVergleich((v) =>
              daten.seite === "a"
                ? { ...v, nodeA: daten.nodeId, modellA: daten.modell }
                : { ...v, nodeB: daten.nodeId, modellB: daten.modell },
            );
          } else if (ereignis === "fertig") {
            setVergleich((v) => ({ ...v, fertig: true }));
          } else if (ereignis === "fehler") {
            setFehler(daten.fehler);
          }
        },
        abbruch.current.signal,
      );
    } catch (e) {
      if (!(e instanceof DOMException && e.name === "AbortError")) {
        setFehler(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setLaufend(false);
      abbruch.current = null;
    }
  }

  async function fassungNehmen(nodeId: number | null) {
    if (nodeId === null) return;
    try {
      await schicke(`/api/nodes/${nodeId}/head`);
      setVergleich(leererVergleich);
      await ladePfad();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center gap-2 border-b border-rand px-3 py-2">
        <Knopf onClick={zurueck} titel="Zur Übersicht">
          ←
        </Knopf>
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm">{story?.title ?? "…"}</div>
          <div className="text-xs text-gedaempft">
            {geld(kosten)} bisher
            {letzterZug && ` · zuletzt ${letzterZug.modell.split("/").pop()} über ${letzterZug.anbieter}`}
          </div>
        </div>
        <Knopf onClick={() => story && promptBearbeiten(story)} titel="System-Prompt">
          Prompt
        </Knopf>
        <Knopf onClick={() => setSeitenleiste((s) => !s)} titel="Speicherstände">
          Stände
        </Knopf>
      </header>

      <div className="flex min-h-0 flex-1">
        <main className="min-w-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-2xl px-4 py-6">
            {knoten.length === 0 && !laufend && (
              <p className="text-sm text-gedaempft">
                Beschreib den Anfang: wo du bist, wer da ist, was du tust. Der Erzähler übernimmt alles
                außer deiner Figur.
              </p>
            )}

            <div className="space-y-6">
              {knoten.map((n) => (
                <article key={n.id} className="group">
                  {n.role === "user" ? (
                    <div className="border-l-2 border-spieler/60 pl-3 text-[0.95rem] text-spieler">
                      {n.content}
                    </div>
                  ) : (
                    <div className="erzaehltext">{n.content}</div>
                  )}
                  <div className="mt-1 flex flex-wrap items-center gap-1 text-xs text-gedaempft opacity-45 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
                    {n.role === "assistant" && n.siblings > 1 && (
                      <span className="mr-1 inline-flex items-center gap-1">
                        <button className="px-1 hover:text-text" onClick={() => fassungWechseln(n.id, -1)}>
                          ‹
                        </button>
                        <span title="Fassungen dieses Zuges">{n.siblings} Fassungen</span>
                        <button className="px-1 hover:text-text" onClick={() => fassungWechseln(n.id, 1)}>
                          ›
                        </button>
                      </span>
                    )}
                    <button className="px-1 hover:text-text" onClick={() => textAendern(n)}>
                      ändern
                    </button>
                    {n.role === "assistant" && (
                      <>
                        <button
                          className="px-1 hover:text-text"
                          disabled={laufend}
                          onClick={() => wiederholen(n.id)}
                        >
                          nochmal
                        </button>
                        {reserveModell && (
                          <button
                            className="px-1 hover:text-text"
                            disabled={laufend}
                            title={`Mit ${reserveModell} wiederholen`}
                            onClick={() => wiederholen(n.id, reserveModell)}
                          >
                            Reservemodell
                          </button>
                        )}
                        <button className="px-1 hover:text-text" onClick={() => setInspektor(n.id)}>
                          was ging raus
                        </button>
                      </>
                    )}
                  </div>
                </article>
              ))}

              {teilText && <div className="erzaehltext schreibt">{teilText}</div>}
              {laufend && !teilText && !vergleich.aktiv && (
                <div className="text-sm text-gedaempft">Der Erzähler denkt nach …</div>
              )}

              {vergleich.aktiv && (
                <div className="space-y-3">
                  <div className="grid gap-3 sm:grid-cols-2">
                    {(["a", "b"] as const).map((seite) => {
                      const text = seite === "a" ? vergleich.textA : vergleich.textB;
                      const modell = seite === "a" ? vergleich.modellA : vergleich.modellB;
                      const node = seite === "a" ? vergleich.nodeA : vergleich.nodeB;
                      return (
                        <div key={seite} className="rounded-lg border border-rand bg-flaeche p-3">
                          <div className="mb-2 truncate text-xs text-gedaempft">
                            {modell || (seite === "a" ? "Erzähler" : "Reserve")}
                          </div>
                          <div className={`erzaehltext ${!node ? "schreibt" : ""}`}>{text}</div>
                          <Knopf
                            klasse="mt-3 w-full"
                            art="haupt"
                            disabled={!node}
                            onClick={() => fassungNehmen(node)}
                          >
                            Diese nehmen
                          </Knopf>
                        </div>
                      );
                    })}
                  </div>
                  <p className="text-xs text-gedaempft">
                    Beide Fassungen bleiben als Zweige erhalten, auch die nicht gewählte.
                  </p>
                </div>
              )}
            </div>

            <Hinweis text={fehler} />
            <div ref={ende} className="h-2" />
          </div>
        </main>

        {seitenleiste && (
          <aside className="w-full max-w-sm shrink-0 overflow-y-auto border-l border-rand bg-flaeche/40 p-4 max-sm:fixed max-sm:inset-y-0 max-sm:right-0 max-sm:z-40 max-sm:bg-grund">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="text-sm tracking-wide text-gedaempft uppercase">Speicherstände</h2>
              <Knopf onClick={() => setSeitenleiste(false)}>✕</Knopf>
            </div>
            <Speicherstaende
              storyId={storyId}
              aktualisierung={slotStand}
              geladen={() => {
                setSeitenleiste(false);
                ladePfad();
              }}
            />
          </aside>
        )}
      </div>

      <footer className="border-t border-rand bg-flaeche/40 px-3 py-3 pb-[env(safe-area-inset-bottom)]">
        <div className="mx-auto flex max-w-2xl gap-2">
          <textarea
            className={`${eingabeKlasse} max-h-40 min-h-[2.75rem] resize-y`}
            rows={2}
            placeholder={
              letzterKnoten?.role === "user" ? "Weiter erzählen lassen …" : "Was tust du?"
            }
            value={eingabe}
            disabled={laufend}
            onChange={(e) => setEingabe(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) senden();
            }}
          />
          <div className="flex flex-col gap-2">
            {laufend ? (
              <Knopf art="gefahr" onClick={() => abbruch.current?.abort()}>
                Stopp
              </Knopf>
            ) : (
              <Knopf art="haupt" onClick={senden}>
                Senden
              </Knopf>
            )}
            <Knopf
              onClick={vergleichStarten}
              disabled={laufend}
              titel="Zwei Modelle nebeneinander auf denselben Prompt"
            >
              A/B
            </Knopf>
          </div>
        </div>
      </footer>

      <Inspektor nodeId={inspektor} schliessen={() => setInspektor(null)} />
    </div>
  );
}
