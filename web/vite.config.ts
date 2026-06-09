/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url';
import { defineConfig, loadEnv } from 'vite';
import type { ProxyOptions } from 'vite';
import vue from '@vitejs/plugin-vue';

// The SPA calls relative /api/... URLs. In dev, Vite proxies them to a running
// `local-ci serve` and injects the bearer token, so the browser stays
// same-origin (no CORS) and the token never lives in client code. Configure the
// target/token in web/.env (see .env.example).
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'VITE_');
  const port = env.VITE_LCI_PORT || '4123';
  const token = env.VITE_LCI_TOKEN || '';
  const target = `http://127.0.0.1:${port}`;

  const apiProxy: ProxyOptions = {
    target,
    changeOrigin: true,
    // SSE needs an unbuffered stream; http-proxy streams by default. The token
    // is injected here (EventSource can't set headers), so SSE to relative
    // /api/... works through the proxy.
    configure: (proxy) => {
      proxy.on('proxyReq', (proxyReq) => {
        if (token) proxyReq.setHeader('Authorization', `Bearer ${token}`);
      });
    },
  };

  return {
    plugins: [vue()],
    resolve: {
      alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
    },
    // Build into the Go tree so `internal/web` can //go:embed it for the
    // single-binary `local-ci ui`. (Dev still uses the Vite server + proxy.)
    build: {
      outDir: fileURLToPath(new URL('../internal/web/dist', import.meta.url)),
      emptyOutDir: true,
    },
    server: {
      proxy: { '/api': apiProxy },
    },
    // Vitest harness is wired up (happy-dom env); tests live in test/*.spec.ts.
    test: {
      globals: true,
      environment: 'happy-dom',
      include: ['test/**/*.spec.ts'],
    },
  };
});
