import { useState } from "react";
import { schicke } from "../api";
import { Hinweis, Knopf, eingabeKlasse } from "./ui";

export default function Anmeldung({
  ersteinrichtung,
  fertig,
}: {
  ersteinrichtung: boolean;
  fertig: () => void;
}) {
  const [passwort, setPasswort] = useState("");
  const [fehler, setFehler] = useState("");
  const [laeuft, setLaeuft] = useState(false);

  async function absenden() {
    setFehler("");
    setLaeuft(true);
    try {
      await schicke(ersteinrichtung ? "/api/setup" : "/api/login", { Passwort: passwort });
      fertig();
    } catch (e) {
      setFehler(e instanceof Error ? e.message : String(e));
    } finally {
      setLaeuft(false);
    }
  }

  return (
    <div className="flex min-h-full items-center justify-center p-6">
      <div className="w-full max-w-sm space-y-4">
        <div>
          <h1 className="text-2xl tracking-tight">Plot</h1>
          <p className="mt-1 text-sm text-gedaempft">
            {ersteinrichtung
              ? "Erster Start: leg ein Passwort fest. Es schützt den Zugang, falls je ein weiteres Gerät ins Tailnet kommt."
              : "Willkommen zurück."}
          </p>
        </div>
        <input
          type="password"
          className={eingabeKlasse}
          placeholder={ersteinrichtung ? "Neues Passwort (min. 8 Zeichen)" : "Passwort"}
          value={passwort}
          autoFocus
          onChange={(e) => setPasswort(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && absenden()}
        />
        <Hinweis text={fehler} />
        <Knopf art="haupt" onClick={absenden} disabled={laeuft || passwort.length < 1} klasse="w-full">
          {ersteinrichtung ? "Einrichten" : "Anmelden"}
        </Knopf>
      </div>
    </div>
  );
}
