/** 快捷键帮助面板深链 */

export type ShortcutsPrefill = {
  open?: boolean
}

function firstQueryValue(v: unknown): string | undefined {
  if (Array.isArray(v)) {
    const x = v[0]
    return typeof x === 'string' && x.trim() ? x.trim() : undefined
  }
  if (typeof v === 'string' && v.trim()) return v.trim()
  return undefined
}

function truthyFlag(v: unknown): boolean {
  const s = firstQueryValue(v)
  if (!s) return false
  const n = s.toLowerCase()
  return n === '1' || n === 'true' || n === 'yes' || n === 'open'
}

export function parseShortcutsPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
  hash?: string,
): ShortcutsPrefill {
  const out: ShortcutsPrefill = {}
  if (truthyFlag(query.shortcuts ?? query.shortcut_help ?? query.help_keys)) {
    out.open = true
  }
  const h = (hash || '').replace(/^#/, '')
  if (h === 'shortcuts' || h === 'shortcuts-help' || h === 'keyboard') {
    out.open = true
  }
  return out
}

/** 解析完整 path（含 query/hash）是否应打开快捷键帮助 */
export function pathOpensShortcutsHelp(path: string | null | undefined): boolean {
  if (!path || typeof path !== 'string') return false
  const raw = path.trim()
  if (!raw.startsWith('/')) return false
  let hash = ''
  let rest = raw
  const hashIdx = raw.indexOf('#')
  if (hashIdx >= 0) {
    hash = raw.slice(hashIdx)
    rest = raw.slice(0, hashIdx)
  }
  const qIdx = rest.indexOf('?')
  const query: Record<string, string> = {}
  if (qIdx >= 0) {
    const qs = rest.slice(qIdx + 1)
    for (const part of qs.split('&')) {
      if (!part) continue
      const eq = part.indexOf('=')
      const k = decodeURIComponent(eq >= 0 ? part.slice(0, eq) : part)
      const v = decodeURIComponent(eq >= 0 ? part.slice(eq + 1) : '')
      if (k) query[k] = v
    }
  }
  return parseShortcutsPrefill(query, hash).open === true
}

export function buildShortcutsPrefillPath(opts?: { path?: string }): string {
  const base = opts?.path && opts.path.startsWith('/') ? opts.path.split('?')[0].split('#')[0] : '/'
  return `${base}?shortcuts=1#shortcuts-help`
}
