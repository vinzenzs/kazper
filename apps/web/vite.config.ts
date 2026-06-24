import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// Same-origin in production (the SPA is embedded in the Kazper binary and served
// at `/`, the API lives under `/api/v1` of the same host — no CORS). In dev the
// Vite server (:5173) proxies `/api` → the local Kazper API (:8080), so the only
// place CORS could matter is dev, and the proxy makes it same-origin there too.
export default defineConfig({
  plugins: [react()],
  // dist is committed and embedded via go:embed (mirrors the docs/ precedent),
  // so `go build` needs no Node toolchain.
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    css: true,
  },
});
