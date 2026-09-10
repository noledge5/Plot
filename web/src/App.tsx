import { useCallback, useEffect, useState } from "react";
import { hole, schicke } from "./api";
import Anmeldung from "./components/Anmeldung";
import Einstellungen from "./components/Einstellungen";
import Geschichten from "./components/Geschichten";
import PromptEditor from "./components/PromptEditor";
import Spiel from "./components/Spiel";
import type { Story, Zustand } from "./types";

export default function App() {
  const [zustand, setZustand] = useState<Zustand | null>(null);
  const [storyId, setStoryId] = useState<number | null>(null);
  const [einstellungenOffen, setEinstellungenOffen] = useState(false);
  const [promptStory, setPromptStory] = useState<Story | null>(null);

  const zustandLaden = useCallback(async () => {
    try {
      setZustand(await hole<Zustand>("/api/state"));
    } catch {
      setZustand({ eingerichtet: false, angemeldet: false, keyGesetzt: false, version: "?", koennen: [] });
    }
  }, []);

  useEffect(() => {
    zustandLaden();
  }, [zustandLaden]);

  if (!zustand) {
    return <div className="p-6 text-sm text-gedaempft">Wird geladen …</div>;
  }

  if (!zustand.angemeldet) {
    return <Anmeldung ersteinrichtung={!zustand.eingerichtet} fertig={zustandLaden} />;
  }

  return (
    <div className="h-full">
      {storyId === null ? (
        <Geschichten
          oeffnen={setStoryId}
          einstellungenOeffnen={() => setEinstellungenOffen(true)}
          keyGesetzt={zustand.keyGesetzt}
          abmelden={async () => {
            await schicke("/api/logout");
            zustandLaden();
          }}
        />
      ) : (
        <Spiel storyId={storyId} zurueck={() => setStoryId(null)} promptBearbeiten={setPromptStory} />
      )}

      <Einstellungen
        offen={einstellungenOffen}
        schliessen={() => setEinstellungenOffen(false)}
        keyGesetzt={zustand.keyGesetzt}
        geaendert={zustandLaden}
      />
      <PromptEditor story={promptStory} schliessen={() => setPromptStory(null)} gespeichert={() => {}} />
    </div>
  );
}
