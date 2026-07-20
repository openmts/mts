/** Retention policy 人类可读 duration → 纳秒 */

export function parseRPDurationToNs(raw: string): number {
  const m = String(raw || '')
    .trim()
    .toLowerCase()
    .match(/^(\d+)(ns|us|ms|s|m|h|d)$/)
  if (!m) {
    const err = new Error('bad duration')
    ;(err as Error & { code?: string }).code = 'bad_duration'
    throw err
  }
  const n = Number(m[1])
  const mul: Record<string, number> = {
    ns: 1,
    us: 1e3,
    ms: 1e6,
    s: 1e9,
    m: 60e9,
    h: 3600e9,
    d: 86400e9,
  }
  const v = n * mul[m[2]]
  if (!Number.isSafeInteger(v)) {
    const err = new Error('overflow')
    ;(err as Error & { code?: string }).code = 'duration_overflow'
    throw err
  }
  return v
}

export function formatRPDuration(ns: number): string {
  if (!ns) return '0'
  if (ns % 86400e9 === 0) return `${ns / 86400e9}d`
  if (ns % 3600e9 === 0) return `${ns / 3600e9}h`
  if (ns % 60e9 === 0) return `${ns / 60e9}m`
  if (ns % 1e9 === 0) return `${ns / 1e9}s`
  return `${ns}ns`
}

export function mapRPDurationError(e: unknown, t: (key: string) => string): string {
  const code = typeof e === 'object' && e && 'code' in e ? String((e as { code?: string }).code || '') : ''
  if (code === 'duration_overflow' || (e instanceof Error && e.message === 'overflow')) {
    return t('databasesErrDurationOverflow')
  }
  return t('databasesErrBadDuration')
}
