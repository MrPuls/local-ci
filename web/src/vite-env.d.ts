/// <reference types="svelte" />
/// <reference types="vite/client" />

interface ImportMetaEnv {
  // Optional absolute API base (e.g. the Tauri sidecar URL). Empty/undefined in
  // browser dev, where the Vite proxy serves /api same-origin.
  readonly VITE_LCI_BASE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
