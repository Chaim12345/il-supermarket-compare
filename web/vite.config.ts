import { defineConfig } from 'vite';

// The Go server serves the built files from web/dist and proxies /api in dev.
export default defineConfig({
  base: './',
  build: { outDir: 'dist', target: 'es2022' },
  server: {
    port: 5173,
    proxy: { '/api': 'http://localhost:8787' },
  },
});
