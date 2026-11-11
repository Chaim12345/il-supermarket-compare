/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL of the Go API. Empty means same origin. */
  readonly VITE_API_BASE?: string;
}
interface ImportMeta {
  readonly env: ImportMetaEnv;
}
