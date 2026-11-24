import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

declare const process: { env: Record<string, string | undefined> };

// The production build is embedded into the Go binary (internal/webui). In development,
// API calls are proxied to a local `trackferry serve` (TRACKFERRY_API, default :18561).
export default defineConfig({
  plugins: [react()],
  build: { outDir: '../internal/webui/dist', emptyOutDir: true, assetsInlineLimit: 0 },
  server: {
    proxy: {
      '/api': process.env.TRACKFERRY_API ?? 'http://127.0.0.1:18561',
      '/healthz': process.env.TRACKFERRY_API ?? 'http://127.0.0.1:18561',
    },
  },
});
