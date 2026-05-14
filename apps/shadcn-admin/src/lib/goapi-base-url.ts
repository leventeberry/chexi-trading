/**
 * Base URL for the Go API (no trailing slash).
 * Empty string means same-origin requests (e.g. Vite dev proxy to `/api`).
 */
export function getGoApiBaseUrl(): string {
  const raw = import.meta.env.VITE_GOAPI_BASE_URL
  if (typeof raw === 'string' && raw.trim() !== '') {
    return raw.replace(/\/+$/, '')
  }
  return ''
}
