export function normalizeAPIBase(base: string | null | undefined): string {
  const normalized = String(base ?? '').trim().replace(/\/+$/, '')
  return normalized === '/' ? '' : normalized
}

export function buildAPIURL(base: string | null | undefined, path: string): string {
  const normalizedBase = normalizeAPIBase(base)
  const normalizedPath = path.startsWith('/') ? path : '/' + path
  return normalizedBase + normalizedPath
}
