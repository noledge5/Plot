// Schmale Hülle um die Backend-Schnittstelle. Fehler kommen als Text zurück,
// wie ihn das Backend formuliert hat - übersetzt wird hier nichts.

async function auswerten<T>(antwort: Response): Promise<T> {
  if (!antwort.ok) {
    let text = `Fehler ${antwort.status}`;
    try {
      const daten = await antwort.json();
      if (daten?.fehler) text = daten.fehler;
    } catch {
      /* keine JSON-Antwort, Statustext genügt */
    }
    throw new Error(text);
  }
  return antwort.json() as Promise<T>;
}

export async function hole<T>(pfad: string): Promise<T> {
  return auswerten<T>(await fetch(pfad));
}

export async function schicke<T>(pfad: string, koerper?: unknown, methode = "POST"): Promise<T> {
  return auswerten<T>(
    await fetch(pfad, {
      method: methode,
      headers: { "Content-Type": "application/json" },
      body: koerper === undefined ? undefined : JSON.stringify(koerper),
    }),
  );
}

export type SSEHandler = (ereignis: string, daten: any) => void;

/**
 * Liest einen Server-Sent-Events-Strom aus einer POST-Antwort. EventSource
 * kann kein POST, deshalb von Hand.
 */
export async function strom(
  pfad: string,
  koerper: unknown,
  bei: SSEHandler,
  signal?: AbortSignal,
): Promise<void> {
  const antwort = await fetch(pfad, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(koerper),
    signal,
  });
  if (!antwort.ok || !antwort.body) {
    await auswerten(antwort); // wirft mit der Meldung des Backends
    return;
  }

  const leser = antwort.body.getReader();
  const decoder = new TextDecoder();
  let puffer = "";

  while (true) {
    const { done, value } = await leser.read();
    if (done) break;
    puffer += decoder.decode(value, { stream: true });

    // Ereignisse sind durch eine Leerzeile getrennt.
    let grenze: number;
    while ((grenze = puffer.indexOf("\n\n")) !== -1) {
      const block = puffer.slice(0, grenze);
      puffer = puffer.slice(grenze + 2);

      let ereignis = "message";
      let nutzlast = "";
      for (const zeile of block.split("\n")) {
        if (zeile.startsWith("event: ")) ereignis = zeile.slice(7).trim();
        else if (zeile.startsWith("data: ")) nutzlast += zeile.slice(6);
      }
      if (!nutzlast) continue;
      try {
        bei(ereignis, JSON.parse(nutzlast));
      } catch {
        /* unvollständige Nutzlast überspringen */
      }
    }
  }
}
