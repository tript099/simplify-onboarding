import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 3100,
    host: true,
    // Same-origin proxy to the backend so the HttpOnly session cookie works in
    // the browser (cross-origin + SameSite=Lax would drop it). Set
    // VITE_USE_MOCK=false to exercise the live simplify-onboarding-service.
    //
    // IMPORTANT: proxy only the API SUB-paths (e.g. /auth/login, /auth/me). The
    // bare "/auth" route is the SPA sign-in PAGE and must be served by Vite, not
    // the backend. The "^" regex keys match "/auth/<sub>" but NOT "/auth" itself.
    proxy: {
      "^/auth/": "http://localhost:8090",
      "^/onb/": "http://localhost:8090",
    },
  },
  preview: {
    port: 3100,
  },
});
