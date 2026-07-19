/** 时间工具：前端安全使用毫秒精度，避免 ns Number 精度问题 */

export function nowUnixMs(): number {
  return Date.now()
}

export function nowUnixMsString(): string {
  return String(Date.now())
}

/** 解析用户输入的整型时间字符串；非法返回 null */
export function parseTimeInt(raw: string): number | null {
  const s = raw.trim()
  if (!s) return null
  if (!/^-?\d+$/.test(s)) return null
  const n = Number(s)
  if (!Number.isSafeInteger(n)) return null
  return n
}

export function formatEpoch(value: number, precision: 'ms' | 'ns' = 'ms'): string {
  if (!Number.isFinite(value)) return '—'
  const ms = precision === 'ns' ? value / 1e6 : value
  try {
    return new Date(ms).toISOString()
  } catch {
    return String(value)
  }
}
