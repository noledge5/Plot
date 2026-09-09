import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    // Das Bundle wandert direkt dorthin, wo das Go-Binary es einbettet.
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
  },
  server: {
    // Für die Entwicklung: das Backend läuft daneben auf 8080.
    proxy: { "/api": "http://localhost:8080" },
  },
});
