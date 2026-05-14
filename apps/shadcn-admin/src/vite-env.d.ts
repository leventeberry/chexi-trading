/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Go API origin (no trailing slash). Omit for same-origin / Vite proxy. */
  readonly VITE_GOAPI_BASE_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
