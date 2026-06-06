/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL for the local-ci API. Empty in dev (same-origin via the Vite proxy) and in Tauri. */
  readonly VITE_LCI_BASE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

declare module '*.vue' {
  import type { DefineComponent } from 'vue';
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>;
  export default component;
}
