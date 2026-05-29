import { defineConfig, loadEnv } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// The SPA calls relative /api/... URLs. In dev, Vite proxies them to a running
// `local-ci serve` and injects the bearer token, so the browser stays
// same-origin (no CORS) and the token never lives in client code. Configure the
// target/token in web/.env (see .env.example).
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'VITE_');
  const port = env.VITE_LCI_PORT || '4123';
  const token = env.VITE_LCI_TOKEN || '';
  const target = `http://127.0.0.1:${port}`;

  return {
    plugins: [svelte()],
    server: {
      proxy: {
        '/api': {
          target,
          changeOrigin: true,
          // SSE needs an unbuffered stream; http-proxy streams by default.
          configure: (proxy: any) => {
            proxy.on('proxyReq', (proxyReq: any) => {
              if (token) {
                proxyReq.setHeader('Authorization', `Bearer ${token}`);
              }
            });
          },
        },
      },
    },
  };
});
