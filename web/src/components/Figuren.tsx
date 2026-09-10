import { useEffect, useState } from "react";
import { hole, schicke } from "../api";
import type { Antrieb, Blatt, Figur, Geheimnis, WeicheGrenze } from "../types";
import { Dialog, Feld, Hinweis, Knopf, eingabeKlasse } from "./ui";

const ACHSEN = ["trust", "warmth", "attraction", "respect", "familiarity", "obligation", "fear"] as const;
const ACHSEN_NAMEN: Record<string, string> = {
  trust: "Vertrauen",
  warmth: "Zuneigung",
  attraction: "Anziehung",
  respect: "Achtung",
  familiarity: "Vertrautheit",
  obligation: "Schuld",
  fear: "Furcht",
};

const leeresBlatt: Blatt = {
  kern: "",
  sprechweise: "",
  drives: [],
  hardLimits: [],
  softLimits: [],
  dealBreakers: [],
  secrets: [],
  volatility: {},
};

/** Sorgt dafür, dass die Listen nie null sind — Go liefert für leere Listen null. */
function normiert(b: Blatt): Blatt {
  return {
    ...leeresBlatt,
    ...b,
    drives: b.drives ?? [],
    hardLimits: b.hardLimits ?? [],
    softLimits: b.softLimits ?? [],
    dealBreakers: b.dealBreakers ?? [],
    secrets: b.secrets ?? [],
    volatility: b.volatility ?? {},
  };
}

function StringListe({
  werte,
  setzen,
  platzhalter,
}: {
  werte: string[];
  setzen: (w: string[]) => void;
  platzhalter: string;
}) {
  return (
    <div className="space-y-2">
      {werte.map((w, i) => (
        <div key={i} className="flex gap-2">
          <input
            className={eingabeKlasse}
            value={w}
            placeholder={platzhalter}
            onChange={(e) => setzen(werte.map((alt, j) => (i === j ? e.target.value : alt)))}
          />
          <Knopf art="gefahr" onClick={() => setzen(werte.filter((_, j) => j !== i))}>
            ✕
          </Knopf>
        </div>
      ))}
      <Knopf onClick={() => setzen([...werte, ""])}>+ Zeile</Knopf>
    </div>
  );
}

function SchwelleWaehler({
  schwellen,
  setzen,
}: {
  schwellen: Record<string, number>;
  setzen: (s: Record<string, number>) => void;
}) {
  const achse = Object.keys(schwellen)[0] ?? "trust";
  const wert = schwellen[achse] ?? 60;
  return (
    <div className="flex flex-wrap items-center gap-2 text-xs text-gedaempft">
      <span>erst wenn</span>
      <select
        className="rounded border border-rand bg-grund px-2 py-1 text-text"
        value={achse}
        onChange={(e) => setzen({ [e.target.value]: wert })}
      >
        {ACHSEN.map((a) => (
          <option key={a} value={a}>
            {ACHSEN_NAMEN[a]}
          </option>
        ))}
      </select>
      <input
        type="range"
        min={0}
        max={100}
        step={5}
        value={wert}
        className="flex-1"
        onChange={(e) => setzen({ [achse]: Number(e.target.value) })}
      />
      <span className="w-8 text-right">{wert}</span>
    </div>
  );
}

export default function Figuren({
  storyId,
  offen,
  schliessen,
  geaendert,
}: {
  storyId: number;
  offen: boolean;
  schliessen: () => void;
  geaendert: () => void;
}) {
  const [figuren, setFiguren] = useState<Figur[]>([]);
  const [auswahl, setAuswahl] = useState<Figur | null>(null);
  const [blatt, setBlatt] = useState<Blatt>(leeresBlatt);
  const [name, setName] = useState("");
  const [aktiv, setAktiv] = useState(true);
  const [fehler, setFehler] = useState("");

  async function laden() {
    try {
      setFiguren(await hole<Figur[]>(`/api/stories/${storyId}/characters`));
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    if (offen) laden();
  }, [offen, storyId]);

  function waehlen(f: Figur) {
    setAuswahl(f);
    setName(f.name);
    setBlatt(normiert(f.sheet));
    setAktiv(f.aktiv);
    setFehler("");
  }

  async function neu(rolle: "npc" | "persona") {
    setFehler("");
    try {
      const f = await schicke<Figur>(`/api/stories/${storyId}/characters`, {
        name: rolle === "persona" ? "Meine Figur" : "Neue Figur",
        rolle,
        sheet: leeresBlatt,
      });
      await laden();
      waehlen(f);
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function speichern() {
    if (!auswahl) return;
    setFehler("");
    try {
      await schicke(
        `/api/characters/${auswahl.id}`,
        { name, sheet: blatt, aktiv, sortierung: auswahl.sortierung },
        "PUT",
      );
      await laden();
      geaendert();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    }
  }

  async function loeschen() {
    if (!auswahl || !window.confirm(`„${auswahl.name}“ löschen?`)) return;
    await schicke(`/api/characters/${auswahl.id}`, undefined, "DELETE");
    setAuswahl(null);
    await laden();
    geaendert();
  }

  const setzeBlatt = (teil: Partial<Blatt>) => setBlatt((b) => ({ ...b, ...teil }));

  return (
    <Dialog titel="Figuren" offen={offen} schliessen={schliessen} breit>
      <div className="grid gap-4 sm:grid-cols-[13rem_1fr]">
        <div className="space-y-2">
          {figuren.map((f) => (
            <button
              key={f.id}
              onClick={() => waehlen(f)}
              className={`block w-full rounded-lg border px-3 py-2 text-left text-sm ${
                auswahl?.id === f.id ? "border-akzent/60 bg-flaeche" : "border-rand hover:border-gedaempft"
              }`}
            >
              <div className="truncate">{f.name}</div>
              <div className="text-xs text-gedaempft">
                {f.rolle === "persona" ? "deine Figur" : f.aktiv ? "in der Szene" : "abwesend"}
              </div>
            </button>
          ))}
          <div className="flex gap-2">
            <Knopf klasse="flex-1" onClick={() => neu("npc")}>
              + Figur
            </Knopf>
            {!figuren.some((f) => f.rolle === "persona") && (
              <Knopf klasse="flex-1" onClick={() => neu("persona")}>
                + Ich
              </Knopf>
            )}
          </div>
        </div>

        {!auswahl ? (
          <p className="text-sm text-gedaempft">
            Wähle links eine Figur oder leg eine neue an. Was hier in den Feldern steht, geht über die
            Platzhalter in deinen System-Prompt — Fließtext im Kern wird von kleinen Modellen eher
            überlesen als eine benannte Liste.
          </p>
        ) : (
          <div className="space-y-5">
            <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
              <Feld label="Name">
                <input className={eingabeKlasse} value={name} onChange={(e) => setName(e.target.value)} />
              </Feld>
              {auswahl.rolle === "npc" && (
                <label className="flex items-end gap-2 pb-2 text-sm">
                  <input type="checkbox" checked={aktiv} onChange={(e) => setAktiv(e.target.checked)} />
                  in der Szene
                </label>
              )}
            </div>

            <Feld label="Kern" hinweis="Zwei, drei Sätze: wer sie ist.">
              <textarea
                className={`${eingabeKlasse} min-h-20`}
                value={blatt.kern}
                onChange={(e) => setzeBlatt({ kern: e.target.value })}
              />
            </Feld>

            <Feld label="Sprechweise" hinweis="Satzbau, Wortwahl, Eigenheiten.">
              <input
                className={eingabeKlasse}
                value={blatt.sprechweise}
                placeholder="kurze Sätze, kein Smalltalk, sagt „hm“ statt „ja“"
                onChange={(e) => setzeBlatt({ sprechweise: e.target.value })}
              />
            </Feld>

            <div className="rounded-lg border border-rand p-3">
              <h3 className="mb-2 text-sm tracking-wide text-gedaempft uppercase">Antriebe</h3>
              <p className="mb-3 text-xs text-gedaempft">
                Was sie von sich aus will. Steigt der Druck über die Schwelle der Geschichte, bringt sie
                es aktiv zur Sprache — das ist der Unterschied zwischen reagierenden und handelnden
                Figuren.
              </p>
              <div className="space-y-3">
                {(blatt.drives ?? []).map((d: Antrieb, i) => (
                  <div key={i} className="space-y-2 rounded border border-rand bg-grund p-2">
                    <div className="flex gap-2">
                      <input
                        className={eingabeKlasse}
                        value={d.ziel}
                        placeholder="die Pacht bis Freitag zusammenbekommen"
                        onChange={(e) =>
                          setzeBlatt({
                            drives: (blatt.drives ?? []).map((alt, j) =>
                              i === j ? { ...alt, ziel: e.target.value } : alt,
                            ),
                          })
                        }
                      />
                      <Knopf
                        art="gefahr"
                        onClick={() =>
                          setzeBlatt({ drives: (blatt.drives ?? []).filter((_, j) => j !== i) })
                        }
                      >
                        ✕
                      </Knopf>
                    </div>
                    <div className="flex flex-wrap items-center gap-2 text-xs text-gedaempft">
                      <span>Druck</span>
                      <input
                        type="range"
                        min={0}
                        max={100}
                        step={5}
                        value={d.druck}
                        className="flex-1"
                        onChange={(e) =>
                          setzeBlatt({
                            drives: (blatt.drives ?? []).map((alt, j) =>
                              i === j ? { ...alt, druck: Number(e.target.value) } : alt,
                            ),
                          })
                        }
                      />
                      <span className="w-8 text-right">{d.druck}</span>
                      <label className="flex items-center gap-1">
                        <input
                          type="checkbox"
                          checked={d.sichtbar}
                          onChange={(e) =>
                            setzeBlatt({
                              drives: (blatt.drives ?? []).map((alt, j) =>
                                i === j ? { ...alt, sichtbar: e.target.checked } : alt,
                              ),
                            })
                          }
                        />
                        man merkt es ihr an
                      </label>
                    </div>
                  </div>
                ))}
                <Knopf
                  onClick={() =>
                    setzeBlatt({ drives: [...(blatt.drives ?? []), { ziel: "", druck: 40, sichtbar: false }] })
                  }
                >
                  + Antrieb
                </Knopf>
              </div>
            </div>

            <div className="rounded-lg border border-rand p-3">
              <h3 className="mb-2 text-sm tracking-wide text-gedaempft uppercase">Grenzen</h3>
              <Feld label="Tut sie niemals" hinweis="Gilt bei jedem Beziehungswert, ausnahmslos.">
                <StringListe
                  werte={blatt.hardLimits ?? []}
                  setzen={(w) => setzeBlatt({ hardLimits: w })}
                  platzhalter="jemanden an die Wache verraten"
                />
              </Feld>

              <div className="mt-4">
                <span className="mb-1 block text-sm text-gedaempft">Gibt erst nach ab einer Schwelle</span>
                <div className="space-y-3">
                  {(blatt.softLimits ?? []).map((sl: WeicheGrenze, i) => (
                    <div key={i} className="space-y-2 rounded border border-rand bg-grund p-2">
                      <div className="flex gap-2">
                        <input
                          className={eingabeKlasse}
                          value={sl.was}
                          placeholder="über ihren Bruder reden"
                          onChange={(e) =>
                            setzeBlatt({
                              softLimits: (blatt.softLimits ?? []).map((alt, j) =>
                                i === j ? { ...alt, was: e.target.value } : alt,
                              ),
                            })
                          }
                        />
                        <Knopf
                          art="gefahr"
                          onClick={() =>
                            setzeBlatt({ softLimits: (blatt.softLimits ?? []).filter((_, j) => j !== i) })
                          }
                        >
                          ✕
                        </Knopf>
                      </div>
                      <SchwelleWaehler
                        schwellen={sl.erstAb ?? {}}
                        setzen={(s) =>
                          setzeBlatt({
                            softLimits: (blatt.softLimits ?? []).map((alt, j) =>
                              i === j ? { ...alt, erstAb: s } : alt,
                            ),
                          })
                        }
                      />
                    </div>
                  ))}
                  <Knopf
                    onClick={() =>
                      setzeBlatt({
                        softLimits: [...(blatt.softLimits ?? []), { was: "", erstAb: { trust: 60 } }],
                      })
                    }
                  >
                    + weiche Grenze
                  </Knopf>
                </div>
              </div>

              <div className="mt-4">
                <Feld label="Nimmt sie dauerhaft übel" hinweis="Senkt Werte drastisch, wenn es eintritt.">
                  <StringListe
                    werte={blatt.dealBreakers ?? []}
                    setzen={(w) => setzeBlatt({ dealBreakers: w })}
                    platzhalter="in ihrer Küche herumschnüffeln"
                  />
                </Feld>
              </div>
            </div>

            <div className="rounded-lg border border-rand p-3">
              <h3 className="mb-2 text-sm tracking-wide text-gedaempft uppercase">Geheimnisse</h3>
              <div className="space-y-3">
                {(blatt.secrets ?? []).map((g: Geheimnis, i) => (
                  <div key={i} className="space-y-2 rounded border border-rand bg-grund p-2">
                    <div className="flex gap-2">
                      <input
                        className={eingabeKlasse}
                        value={g.text}
                        placeholder="dass sie den Brief gelesen hat"
                        onChange={(e) =>
                          setzeBlatt({
                            secrets: (blatt.secrets ?? []).map((alt, j) =>
                              i === j ? { ...alt, text: e.target.value } : alt,
                            ),
                          })
                        }
                      />
                      <Knopf
                        art="gefahr"
                        onClick={() =>
                          setzeBlatt({ secrets: (blatt.secrets ?? []).filter((_, j) => j !== i) })
                        }
                      >
                        ✕
                      </Knopf>
                    </div>
                    <SchwelleWaehler
                      schwellen={g.preisgabeAb ?? {}}
                      setzen={(s) =>
                        setzeBlatt({
                          secrets: (blatt.secrets ?? []).map((alt, j) =>
                            i === j ? { ...alt, preisgabeAb: s } : alt,
                          ),
                        })
                      }
                    />
                  </div>
                ))}
                <Knopf
                  onClick={() =>
                    setzeBlatt({
                      secrets: [...(blatt.secrets ?? []), { text: "", preisgabeAb: { trust: 80 } }],
                    })
                  }
                >
                  + Geheimnis
                </Knopf>
              </div>
            </div>

            <Hinweis text={fehler} />
            <div className="flex justify-between">
              <Knopf art="gefahr" onClick={loeschen}>
                Löschen
              </Knopf>
              <Knopf art="haupt" onClick={speichern}>
                Speichern
              </Knopf>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
}
