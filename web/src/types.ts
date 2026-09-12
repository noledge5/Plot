export type Zustand = {
  eingerichtet: boolean;
  angemeldet: boolean;
  keyGesetzt: boolean;
  version: string;
  /** Was dieser Stand kann — damit man sieht, ob ein Update angekommen ist. */
  koennen: string[];
};

export type StorySettings = {
  stilbeispiel: string;
  szene: string;
  welt: string;
  autorNotiz: string;
  druckSchwelle: number;
};

export type Story = {
  id: number;
  title: string;
  systemPrompt: string;
  settings: StorySettings;
  headNodeId: number | null;
  createdAt: string;
  updatedAt: string;
};

export type Antrieb = { ziel: string; druck: number; sichtbar: boolean };
export type WeicheGrenze = { was: string; erstAb: Record<string, number> };
export type Geheimnis = { text: string; preisgabeAb: Record<string, number> };

export type Blatt = {
  kern: string;
  sprechweise: string;
  verhaeltnis: string;
  beziehung: Record<string, number> | null;
  drives: Antrieb[] | null;
  hardLimits: string[] | null;
  softLimits: WeicheGrenze[] | null;
  dealBreakers: string[] | null;
  secrets: Geheimnis[] | null;
  volatility: Record<string, number> | null;
};

export type Figur = {
  id: number;
  storyId: number;
  rolle: "npc" | "persona";
  name: string;
  sheet: Blatt;
  aktiv: boolean;
  sortierung: number;
  createdAt: string;
  updatedAt: string;
};

export type Befund = { muster: number; kurz: string; zitat: string; grund: string };
export type Flags = { geprueft: boolean; modell?: string; befunde: Befund[] | null; fehler?: string };

export type PromptBlock = { platzhalter: string; inhalt: string; tokens: number };

export type Knoten = {
  id: number;
  storyId: number;
  parentId: number | null;
  kind: string;
  role: "user" | "assistant";
  content: string;
  model: string;
  usage: string;
  flags: string;
  createdAt: string;
  siblings: number;
};

export type Slot = {
  id: number;
  storyId: number;
  nodeId: number;
  name: string;
  kind: "manual" | "auto";
  preview: string;
  createdAt: string;
};

export type Modell = {
  id: string;
  name: string;
  contextLength: number;
  promptPerM: number;
  outputPerM: number;
  strictJson: boolean;
};

export type Einstellungen = {
  detektorAn: boolean;
  beziehungenAn: boolean;
  gedaechtnisAn: boolean;
  faktenAutomatisch: boolean;
  narratorModel: string;
  reserveModel: string;
  analystModel: string;
  narratorProviders: string[];
  narratorZdr: boolean;
  analystZdr: boolean;
  maxHistoryTurns: number;
  maxTokens: number;
  temperature: number;
};

export type Verbrauch = {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost: number;
};

export type Inspektion = { erzaehlung: Protokoll; alle: Protokoll[] };

export type Protokoll = {
  id: number;
  nodeId: number | null;
  purpose: string;
  model: string;
  request: string;
  response: string;
  costUsd: number;
  latencyMs: number;
  error: string;
  createdAt: string;
};

export type Fakt = {
  id: number;
  storyId: number;
  betrifft: string;
  text: string;
  gewicht: number;
  status: "proposed" | "canon" | "retired";
  angeheftet: boolean;
  quelle: number | null;
  giltAb: number | null;
  giltBis: number | null;
  createdAt: string;
  updatedAt: string;
};

export type Abschnitt = {
  id: number;
  storyId: number;
  ebene: number;
  vonNode: number;
  bisNode: number;
  text: string;
  createdAt: string;
};

export type Gedaechtnis = { chronik: Abschnitt[]; imPrompt: Fakt[]; bisKnoten: number };
