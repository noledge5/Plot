export type Zustand = { eingerichtet: boolean; angemeldet: boolean; keyGesetzt: boolean };

export type Story = {
  id: number;
  title: string;
  systemPrompt: string;
  settings: string;
  headNodeId: number | null;
  createdAt: string;
  updatedAt: string;
};

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
