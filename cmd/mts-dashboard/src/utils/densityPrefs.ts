/** 界面密度偏好（localStorage） */

export const DENSITY_PREFS_KEY = 'mts.dashboard.density.prefs.v1'

export type UiDensity = 'comfortable' | 'compact'

export const DEFAULT_UI_DENSITY: UiDensity = 'comfortable'

export function normalizeUiDensity(raw: unknown): UiDensity {
  return raw === 'compact' ? 'compact' : 'comfortable'
}

export function loadUiDensity(
  storage: Pick<Storage, 'getItem'> | null,
  key = DENSITY_PREFS_KEY,
): UiDensity {
  if (!storage) return DEFAULT_UI_DENSITY
  try {
    const raw = storage.getItem(key)
    if (!raw) return DEFAULT_UI_DENSITY
    if (raw.startsWith('{')) {
      const o = JSON.parse(raw) as { density?: unknown }
      return normalizeUiDensity(o.density)
    }
    return normalizeUiDensity(raw)
  } catch {
    return DEFAULT_UI_DENSITY
  }
}

export function saveUiDensity(
  storage: Pick<Storage, 'setItem' | 'removeItem'> | null,
  density: UiDensity,
  key = DENSITY_PREFS_KEY,
): void {
  if (!storage) return
  try {
    const d = normalizeUiDensity(density)
    if (d === DEFAULT_UI_DENSITY) {
      storage.removeItem(key)
      return
    }
    storage.setItem(key, JSON.stringify({ version: 1, density: d }))
  } catch {
    /* ignore */
  }
}

export function applyUiDensity(
  density: UiDensity,
  root: { setAttribute: (n: string, v: string) => void; removeAttribute: (n: string) => void } | null,
): void {
  if (!root) return
  const d = normalizeUiDensity(density)
  if (d === 'compact') root.setAttribute('data-density', 'compact')
  else root.removeAttribute('data-density')
}
