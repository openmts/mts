/** 将用户友好的 duration（如 1m/5m/1h/1d）解析为纳秒 */
export function parseHumanDurationToNs(input: string): number {
  const s = input.trim().toLowerCase()
  if (!s) throw new Error('interval 不能为空')
  const m = s.match(/^(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h|d)$/)
  if (!m) throw new Error('interval 格式无效，示例：30s / 1m / 5m / 1h / 1d')
  const n = Number(m[1])
  if (!Number.isFinite(n) || n <= 0) throw new Error('interval 必须为正数')
  const unit = m[2]
  const mul: Record<string, number> = {
    ns: 1,
    us: 1e3,
    'µs': 1e3,
    ms: 1e6,
    s: 1e9,
    m: 60e9,
    h: 3600e9,
    d: 86400e9,
  }
  const ns = n * mul[unit]
  if (!Number.isSafeInteger(ns)) throw new Error('interval 超出安全整数范围')
  return ns
}

export function formatNsDuration(ns: number): string {
  if (!Number.isFinite(ns) || ns <= 0) return String(ns)
  if (ns % 86400e9 === 0) return `${ns / 86400e9}d`
  if (ns % 3600e9 === 0) return `${ns / 3600e9}h`
  if (ns % 60e9 === 0) return `${ns / 60e9}m`
  if (ns % 1e9 === 0) return `${ns / 1e9}s`
  if (ns % 1e6 === 0) return `${ns / 1e6}ms`
  return `${ns}ns`
}
