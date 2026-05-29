// A tiny reactive hash router. SPA/Tauri-friendly (no history API, no deps).
// Routes: "#/" → list, "#/runs/:id" → detail.

export type Route = { name: 'list' } | { name: 'detail'; id: string };

function parse(): Route {
  const hash = location.hash.replace(/^#/, '');
  const m = hash.match(/^\/runs\/(.+)$/);
  if (m) return { name: 'detail', id: decodeURIComponent(m[1]) };
  return { name: 'list' };
}

// Exported as an object property so importers observe updates reactively.
export const router = $state<{ route: Route }>({ route: parse() });

let started = false;
export function startRouter(): void {
  if (started) return;
  started = true;
  window.addEventListener('hashchange', () => {
    router.route = parse();
  });
}

export function navigate(hash: string): void {
  location.hash = hash;
}
