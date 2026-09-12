import { useEffect, useRef, useState } from "react";
import { hole, schicke, strom } from "../api";
import type { Einstellungen, Flags, Knoten, Story, Verbrauch } from "../types";
import Figuren from "./Figuren";
import Gedaechtnis from "./Gedaechtnis";
import Inspektor from "./Inspektor";
import Speicherstaende from "./Speicherstaende";
import { Hinweis, Knopf, eingabeKlasse, geld } from "./ui";

type PfadAntwort = { story: Story; knoten: Knoten[]; kosten: number; aufrufe: number };

type Modus = "handlung" | "dialog" | "regie";

const MODI: { wert: Modus; name: string; hilfe: string }[] = [
  { wert: "handlung", name: "Handlung", hilfe: "Was deine Figur tut" },
  {
    wert: "dialog",
    name: "Gesagt",
    hilfe: "Was deine Figur laut sagt — geht in Anführungszeichen raus",
  },
  {
    wert: "regie",
    name: "Regie",
    hilfe: "Anweisung an den Erzähler — steht nicht in der Geschichte",
  },
];

/** Der zuletzt benutzte Modus überlebt einen Seitenwechsel. */
const gemerkterModus: Modus = (() => {
  try {
    const m = localStorage.getItem("plot.modus");
    return m === "dialog" || m === "regie" ? m : "handlung";
  } catch {
    return "handlung";
  }
})();

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
  const [figurenOffen, setFigurenOffen] = useState(false);
  const [gedaechtnisOffen, setGedaechtnisOffen] = useState(false);
  // Kurzfassung beim Öffnen: wo man steht, wer da ist, was offen war.
  const [einstieg, setEinstieg] = useState<{
    chronik: string;
    zuletzt: string;
    anwesend: string[];
    wichtiges: { id: number; text: string }[];
    zuege: number;
  } | null>(null);
  const [einstiegZu, setEinstiegZu] = useState(false);
  // "regie" schickt den Text als Anweisung an den Erzähler statt als Handlung
  // der Spielerfigur, "dialog" als wörtliche Rede in Anführungszeichen.
  const [modus, setModus] = useState<Modus>(gemerkterModus);
  // Was gerade abgeschickt wurde, steht sofort im Verlauf - sonst verschwindet
  // der eigene Satz aus dem Feld und taucht erst wieder auf, wenn der Erzähler
  // fertig ist. Bei einem kleinen Modell sind das zwanzig Sekunden, in denen
  // man glaubt, die Eingabe sei verloren gegangen.
  const [unterwegs, setUnterwegs] = useState<{ text: string; modus: Modus } | null>(null);
  // Anweisungen, die bei jedem Zug mitgehen, bis man sie wegnimmt.
  const [regeln, setRegeln] = useState<string[]>([]);
  const [regelnOffen, setRegelnOffen] = useState(false);
  // Befunde des Detektors je Knoten. Sie kommen beim Laden aus dem Verlauf und
  // während eines Zuges als eigenes Ereignis nach.
  const [befunde, setBefunde] = useState<Record<number, Flags>>({});

  const abbruch = useRef<AbortController | null>(null);
  const ende = useRef<HTMLDivElement>(null);

  async function ladePfad() {
    try {
      const d = await hole<PfadAntwort>(`/api/stories/${storyId}/path`);
      setStory(d.story);
      setKnoten(d.knoten);
      setKosten(d.kosten);
      const gefunden: Record<number, Flags> = {};
      for (const n of d.knoten) {
        if (!n.flags || n.flags === "{}") continue;
        try {
          gefunden[n.id] = JSON.parse(n.flags) as Flags;
        } catch {
          /* unlesbare Befunde einfach weglassen */
        }
      }
      setBefunde(gefunden);
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function ladeRegeln() {
    try {
      setRegeln((await hole<string[] | null>(`/api/stories/${storyId}/regie-regeln`)) ?? []);
    } catch {
      setRegeln([]);
    }
  }

  useEffect(() => {
    ladePfad();
    ladeRegeln();
    setEinstiegZu(false);
    hole<typeof einstieg>(`/api/stories/${storyId}/wiedereinstieg`)
      .then((d) => setEinstieg(d && d.zuege > 2 ? d : null))
      .catch(() => setEinstieg(null));
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
  }, [knoten.length, teilText, unterwegs, vergleich.textA, vergleich.textB]);

  useEffect(() => {
    try {
      localStorage.setItem("plot.modus", modus);
    } catch {
      /* privater Modus ohne Speicher - dann eben nicht */
    }
  }, [modus]);

  const letzterKnoten = knoten.length ? knoten[knoten.length - 1] : null;

  async function senden() {
    const text = eingabe.trim();
    if (!text && knoten.length === 0) return;
    setFehler("");
    setTeilText("");
    setLaufend(true);
    setEingabe("");
    setUnterwegs(text ? { text, modus } : null);
    abbruch.current = new AbortController();
    try {
      await strom(
        `/api/stories/${storyId}/turn`,
        { text, modus },
        (ereignis, daten) => {
          if (ereignis === "delta") setTeilText((t) => t + daten.text);
          else if (ereignis === "fehler") setFehler(daten.fehler);
          else if (ereignis === "fertig") {
            setLetzterZug({ modell: daten.modell, anbieter: daten.anbieter, usage: daten.usage });
          } else if (ereignis === "befunde") {
            setBefunde((b) => ({ ...b, [daten.nodeId]: daten.flags }));
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
      // Erst neu laden, dann die Vorschau wegnehmen: sonst fehlt der eigene
      // Satz für einen Wimpernschlag ganz.
      await ladePfad();
      setUnterwegs(null);
      setSlotStand((n) => n + 1);
    }
  }

  async function wiederholen(nodeId: number, modell?: string, haerter = false) {
    setFehler("");
    setTeilText("");
    setLaufend(true);
    abbruch.current = new AbortController();
    try {
      await strom(
        `/api/stories/${storyId}/regenerate`,
        { nodeId, modell, haerter },
        (ereignis, daten) => {
          if (ereignis === "delta") setTeilText((t) => t + daten.text);
          else if (ereignis === "fehler") setFehler(daten.fehler);
          else if (ereignis === "fertig")
            setLetzterZug({ modell: daten.modell, anbieter: daten.anbieter, usage: daten.usage });
          else if (ereignis === "befunde") setBefunde((b) => ({ ...b, [daten.nodeId]: daten.flags }));
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
    setUnterwegs(text ? { text, modus: "handlung" } : null);
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
      // Der Vergleich legt die Eingabe als Knoten an. Ohne Nachladen bliebe
      // sie im Verlauf unsichtbar, bis eine Fassung gewählt wird.
      await ladePfad();
      setUnterwegs(null);
    }
  }

  /**
   * Nimmt die letzte Erzählung zurück und lässt sie mit dieser Anweisung neu
   * erzählen. Eine gewöhnliche Regie gilt erst für den nächsten Zug — die
   * falsche Stelle bliebe stehen, und das Modell baute weiter darauf auf.
   */
  async function korrigieren() {
    const text = eingabe.trim();
    if (!text) return;
    setFehler("");
    setTeilText("");
    setLaufend(true);
    setEingabe("");
    setUnterwegs({ text, modus: "regie" });
    abbruch.current = new AbortController();
    try {
      await strom(
        `/api/stories/${storyId}/korrektur`,
        { text },
        (ereignis, daten) => {
          if (ereignis === "delta") setTeilText((t) => t + daten.text);
          else if (ereignis === "fehler") setFehler(daten.fehler);
          else if (ereignis === "fertig")
            setLetzterZug({ modell: daten.modell, anbieter: daten.anbieter, usage: daten.usage });
          else if (ereignis === "befunde") setBefunde((b) => ({ ...b, [daten.nodeId]: daten.flags }));
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
      setUnterwegs(null);
      setSlotStand((n) => n + 1);
    }
  }

  /** Legt die Anweisung als stehende Regel ab — sie geht dann bei jedem Zug mit. */
  async function regelAnlegen() {
    const text = eingabe.trim();
    if (!text) return;
    setFehler("");
    try {
      setRegeln(await schicke<string[]>(`/api/stories/${storyId}/regie-regeln`, { text }));
      setEingabe("");
      setRegelnOffen(true);
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function regelLoeschen(nr: number) {
    try {
      setRegeln(await schicke<string[]>(`/api/stories/${storyId}/regie-regeln/${nr}`, undefined, "DELETE"));
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function zeitsprung() {
    const spanne = window.prompt("Wie viel Zeit vergeht?", "drei Tage");
    if (!spanne) return;
    setFehler("");
    setLaufend(true);
    try {
      await schicke(`/api/stories/${storyId}/zeitsprung`, { spanne });
      await ladePfad();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    } finally {
      setLaufend(false);
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
      {/* Auf dem Handy passen Titel und vier Knöpfe nicht in eine Zeile — bei
          390px lief die Kopfzeile aus dem Bild und "Stände" war abgeschnitten.
          Deshalb zwei Zeilen unter sm, eine darüber. */}
      <header className="border-b border-rand px-3 py-2">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center gap-x-2 gap-y-1">
          <Knopf onClick={zurueck} titel="Zur Übersicht">
            ←
          </Knopf>
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm">{story?.title ?? "…"}</div>
            <div className="truncate text-xs text-gedaempft">
              {geld(kosten)} bisher
              {letzterZug && ` · zuletzt ${letzterZug.modell.split("/").pop()} über ${letzterZug.anbieter}`}
            </div>
          </div>
          <div className="flex shrink-0 gap-1 max-sm:w-full max-sm:justify-between">
            <Knopf klasse="max-sm:flex-1 max-sm:px-2 max-sm:text-xs" onClick={() => setFigurenOffen(true)} titel="Figuren, Grenzen, Antriebe">
              Figuren
            </Knopf>
            <Knopf klasse="max-sm:flex-1 max-sm:px-2 max-sm:text-xs" onClick={() => setGedaechtnisOffen(true)} titel="Chronik und Fakten">
              Gedächtnis
            </Knopf>
            <Knopf klasse="max-sm:flex-1 max-sm:px-2 max-sm:text-xs" onClick={() => story && promptBearbeiten(story)} titel="System-Prompt">
              Prompt
            </Knopf>
            <Knopf klasse="max-sm:flex-1 max-sm:px-2 max-sm:text-xs" onClick={() => setSeitenleiste((s) => !s)} titel="Speicherstände">
              Stände
            </Knopf>
          </div>
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        <main className="min-w-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-2xl px-4 py-6">
            {einstieg && !einstiegZu && (
              <div className="mb-6 rounded-lg border border-rand bg-flaeche/60 p-4">
                <div className="mb-2 flex items-center justify-between gap-2">
                  <h2 className="text-sm tracking-wide text-gedaempft uppercase">Wo du stehst</h2>
                  <button
                    className="text-xs text-gedaempft hover:text-text"
                    onClick={() => setEinstiegZu(true)}
                  >
                    ausblenden
                  </button>
                </div>
                {einstieg.chronik && (
                  <p className="erzaehltext mb-2 text-sm text-text/80">{einstieg.chronik}</p>
                )}
                {einstieg.zuletzt && (
                  <p className="erzaehltext mb-2 line-clamp-3 text-sm text-text/70">
                    Zuletzt: {einstieg.zuletzt}
                  </p>
                )}
                <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-gedaempft">
                  {einstieg.anwesend.length > 0 && <span>In der Szene: {einstieg.anwesend.join(", ")}</span>}
                  <span>{einstieg.zuege} Züge bisher</span>
                </div>
                {einstieg.wichtiges.length > 0 && (
                  <ul className="mt-2 space-y-0.5 text-xs text-akzent/80">
                    {einstieg.wichtiges.map((f) => (
                      <li key={f.id}>• {f.text}</li>
                    ))}
                  </ul>
                )}
              </div>
            )}

            {knoten.length === 0 && !laufend && (
              <p className="text-sm text-gedaempft">
                Beschreib den Anfang: wo du bist, wer da ist, was du tust. Der Erzähler übernimmt alles
                außer deiner Figur.
              </p>
            )}

            <div className="space-y-6">
              {knoten.map((n) => (
                <article key={n.id} className="group">
                  {n.kind === "timeskip" ? (
                    <div className="erzaehltext border-y border-rand/60 py-3 text-sm text-text/70">
                      {n.content}
                    </div>
                  ) : n.role === "user" ? (
                    <Eigenes text={n.content} modus={alsModus(n.kind)} />
                  ) : (
                    <div className="erzaehltext">{n.content}</div>
                  )}
                  {n.role === "assistant" && (befunde[n.id]?.befunde?.length ?? 0) > 0 && (
                    <div className="mt-2 space-y-1 rounded-lg border border-amber-800/40 bg-amber-950/20 p-2">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span className="text-xs tracking-wide text-amber-200/80 uppercase">
                          Gefälligkeit erkannt
                        </span>
                        <button
                          className="rounded border border-amber-700/50 px-2 py-0.5 text-xs text-amber-100 hover:border-amber-500 disabled:opacity-40"
                          disabled={laufend}
                          onClick={() => wiederholen(n.id, undefined, true)}
                        >
                          nochmal, härter
                        </button>
                      </div>
                      {befunde[n.id]!.befunde!.map((b, i) => (
                        <div key={i} className="text-xs text-amber-100/80">
                          <span className="font-medium">{b.kurz}:</span> {b.grund}
                          {b.zitat && <span className="text-amber-100/50"> „{b.zitat}“</span>}
                        </div>
                      ))}
                    </div>
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

              {unterwegs && <Eigenes text={unterwegs.text} modus={unterwegs.modus} blass />}
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

      <footer className="border-t border-rand bg-flaeche/40 px-3 py-2 pb-[max(0.5rem,env(safe-area-inset-bottom))]">
        <div className="mx-auto max-w-2xl space-y-2">
          <div className="flex flex-wrap items-center gap-1 text-xs">
            {MODI.map((m) => (
              <button
                key={m.wert}
                onClick={() => setModus(m.wert)}
                className={`rounded px-2 py-1 transition-colors ${
                  modus === m.wert
                    ? m.wert === "regie"
                      ? "bg-akzent/15 text-akzent"
                      : "bg-flaeche text-spieler"
                    : "text-gedaempft hover:text-text"
                }`}
                title={m.hilfe}
              >
                {m.name}
              </button>
            ))}
            {regeln.length > 0 && (
              <button
                className="ml-auto rounded px-2 py-1 text-akzent/70 hover:text-akzent"
                onClick={() => setRegelnOffen((o) => !o)}
                title="Anweisungen, die bei jedem Zug mitgehen"
              >
                {regeln.length} dauerhaft
              </button>
            )}
          </div>

          {regelnOffen && regeln.length > 0 && (
            <ul className="space-y-1 rounded-lg border border-akzent/25 bg-akzent/5 p-2">
              {regeln.map((r, i) => (
                <li key={i} className="flex items-start gap-2 text-xs text-akzent/80">
                  <span className="min-w-0 flex-1">{r}</span>
                  <button
                    className="shrink-0 text-gedaempft hover:text-red-300"
                    title="Anweisung aufheben"
                    onClick={() => regelLoeschen(i)}
                  >
                    ✕
                  </button>
                </li>
              ))}
            </ul>
          )}

          <textarea
            className={`${eingabeKlasse} max-h-40 min-h-[3.25rem] w-full resize-y`}
            rows={2}
            placeholder={
              modus === "regie"
                ? "Mira ist deine Freundin, sie würde dich nicht so abweisen."
                : modus === "dialog"
                  ? "Ist noch Kaffee da?"
                  : letzterKnoten?.role === "user"
                    ? "Weiter erzählen lassen …"
                    : "Was tust du?"
            }
            value={eingabe}
            disabled={laufend}
            onChange={(e) => setEingabe(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) senden();
            }}
          />

          {/* Regie hat drei Wirkungen. Sie stehen nebeneinander, weil der
              Unterschied — nächster Zug, letzter Zug, dauerhaft — die ganze
              Bedienung ausmacht und sich hinter einem Menü nicht erklärt. */}
          {modus === "regie" ? (
            <div className="flex flex-wrap gap-2">
              {laufend ? (
                <Knopf art="gefahr" klasse="flex-1" onClick={() => abbruch.current?.abort()}>
                  Stopp
                </Knopf>
              ) : (
                <>
                  <Knopf
                    art="haupt"
                    klasse="flex-1"
                    onClick={senden}
                    disabled={!eingabe.trim()}
                    titel="Gilt für den nächsten Zug"
                  >
                    Ab jetzt
                  </Knopf>
                  <Knopf
                    klasse="flex-1"
                    onClick={korrigieren}
                    disabled={!eingabe.trim() || letzterKnoten?.role !== "assistant"}
                    titel="Verwirft die letzte Erzählung und erzählt sie mit dieser Anweisung neu"
                  >
                    Letztes neu
                  </Knopf>
                  <Knopf
                    klasse="flex-1"
                    onClick={regelAnlegen}
                    disabled={!eingabe.trim()}
                    titel="Geht bei jedem Zug mit, bis du sie aufhebst"
                  >
                    Dauerhaft
                  </Knopf>
                </>
              )}
            </div>
          ) : (
            <div className="flex flex-wrap gap-2">
              {laufend ? (
                <Knopf art="gefahr" klasse="flex-1" onClick={() => abbruch.current?.abort()}>
                  Stopp
                </Knopf>
              ) : (
                <Knopf art="haupt" klasse="flex-1" onClick={senden} disabled={!eingabe.trim()}>
                  Senden
                </Knopf>
              )}
              <Knopf
                onClick={senden}
                disabled={laufend || knoten.length === 0 || eingabe.trim() !== ""}
                titel="Weitererzählen lassen, ohne selbst etwas beizutragen"
              >
                Weiter
              </Knopf>
              <Knopf onClick={zeitsprung} disabled={laufend || knoten.length === 0} titel="Zeit vergehen lassen">
                Zeit
              </Knopf>
              <Knopf
                onClick={vergleichStarten}
                disabled={laufend}
                titel="Zwei Modelle nebeneinander auf denselben Prompt"
              >
                A/B
              </Knopf>
            </div>
          )}
        </div>
      </footer>

      <Figuren
        storyId={storyId}
        offen={figurenOffen}
        schliessen={() => setFigurenOffen(false)}
        geaendert={ladePfad}
      />
      <Gedaechtnis
        storyId={storyId}
        offen={gedaechtnisOffen}
        schliessen={() => setGedaechtnisOffen(false)}
      />
      <Inspektor nodeId={inspektor} schliessen={() => setInspektor(null)} />
    </div>
  );
}

/** Die Art eines Knotens auf einen Eingabemodus abbilden. */
function alsModus(kind: string): Modus {
  return kind === "regie" || kind === "dialog" ? kind : "handlung";
}

/**
 * Alles, was der Spieler selbst geschrieben hat. Handlung, Gesagtes und Regie
 * sehen bewusst deutlich verschieden aus: Ohne Marke war im Verlauf nicht zu
 * erkennen, was von einem selbst stammt und was der Erzähler geschrieben hat.
 *
 * `blass` ist die Fassung, die während des Zuges steht, bevor der Knoten
 * wirklich gespeichert ist.
 */
function Eigenes({ text, modus, blass = false }: { text: string; modus: Modus; blass?: boolean }) {
  const trueb = blass ? "opacity-60" : "";
  if (modus === "regie") {
    return (
      <div className={`border-l-2 border-akzent/40 pl-3 ${trueb}`}>
        <div className="text-[0.7rem] tracking-widest text-akzent/60 uppercase">Regie</div>
        <div className="text-[0.9rem] text-akzent/80 italic">{text}</div>
      </div>
    );
  }
  if (modus === "dialog") {
    return (
      <div className={`border-l-2 border-spieler pl-3 ${trueb}`}>
        <div className="text-[0.7rem] tracking-widest text-spieler/60 uppercase">Du sagst</div>
        <div className="text-[1rem] text-spieler">{inAnfuehrung(text)}</div>
      </div>
    );
  }
  return (
    <div className={`border-l-2 border-spieler/50 pl-3 ${trueb}`}>
      <div className="text-[0.7rem] tracking-widest text-spieler/50 uppercase">Du</div>
      <div className="text-[0.95rem] text-spieler/90">{text}</div>
    </div>
  );
}

/** Setzt deutsche Anführungszeichen, wenn der Satz noch keine hat. */
function inAnfuehrung(text: string) {
  const t = text.trim();
  const paare = [
    ["\u201e", "\u201c"],
    ['"', '"'],
    ["\u00bb", "\u00ab"],
  ];
  for (const [auf, zu] of paare) {
    if (t.startsWith(auf) && t.endsWith(zu) && t.length > auf.length + zu.length - 1) return t;
  }
  return `\u201e${t}\u201c`;
}
