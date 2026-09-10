import type { ReactNode } from "react";

export function Knopf({
  children,
  onClick,
  art = "still",
  disabled,
  titel,
  klasse = "",
}: {
  children: ReactNode;
  onClick?: () => void;
  art?: "haupt" | "still" | "gefahr";
  disabled?: boolean;
  titel?: string;
  klasse?: string;
}) {
  const arten = {
    haupt: "bg-akzent/90 text-grund hover:bg-akzent font-medium",
    still: "bg-flaeche border border-rand text-text hover:border-gedaempft",
    gefahr: "bg-flaeche border border-rand text-red-300 hover:border-red-500/60",
  };
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={titel}
      className={`rounded-lg px-3 py-2 text-sm transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${arten[art]} ${klasse}`}
    >
      {children}
    </button>
  );
}

export function Feld({
  label,
  hinweis,
  children,
}: {
  label: string;
  hinweis?: string;
  children: ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm text-gedaempft">{label}</span>
      {children}
      {hinweis && <span className="mt-1 block text-xs text-gedaempft/80">{hinweis}</span>}
    </label>
  );
}

export const eingabeKlasse =
  "w-full rounded-lg border border-rand bg-grund px-3 py-2 text-text outline-none focus:border-akzent/60";

export function Dialog({
  titel,
  offen,
  schliessen,
  breit,
  children,
}: {
  titel: string;
  offen: boolean;
  schliessen: () => void;
  breit?: boolean;
  children: ReactNode;
}) {
  if (!offen) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-0 sm:items-center sm:p-4"
      onClick={schliessen}
    >
      <div
        className={`flex max-h-[92vh] w-full flex-col overflow-hidden rounded-t-2xl border border-rand bg-flaeche sm:rounded-2xl ${
          breit ? "sm:max-w-4xl" : "sm:max-w-xl"
        }`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-rand px-4 py-3">
          <h2 className="text-sm font-medium tracking-wide text-gedaempft uppercase">{titel}</h2>
          <button
            onClick={schliessen}
            className="rounded px-2 py-1 text-gedaempft hover:text-text"
            aria-label="Schließen"
          >
            ✕
          </button>
        </div>
        <div className="overflow-y-auto px-4 py-4">{children}</div>
      </div>
    </div>
  );
}

export function Hinweis({ text, art = "fehler" }: { text: string; art?: "fehler" | "gut" }) {
  if (!text) return null;
  return (
    <div
      className={`rounded-lg border px-3 py-2 text-sm ${
        art === "gut"
          ? "border-emerald-700/50 bg-emerald-900/20 text-emerald-200"
          : "border-red-800/60 bg-red-950/30 text-red-200"
      }`}
    >
      {text}
    </div>
  );
}

/** Kosten sind winzig; Cent mit drei Nachkommastellen sind hier ehrlicher als Dollar. */
export function geld(betragUSD: number): string {
  if (betragUSD === 0) return "0 ¢";
  const cent = betragUSD * 100;
  if (cent < 1) return `${cent.toFixed(3)} ¢`;
  if (cent < 100) return `${cent.toFixed(2)} ¢`;
  return `${betragUSD.toFixed(2)} $`;
}
