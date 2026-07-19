/** 表单脏状态：稳定序列化后比较 */

export function stableStringify(value: unknown): string {
  return JSON.stringify(sortKeys(value))
}

function sortKeys(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(sortKeys)
  if (value && typeof value === 'object') {
    const o = value as Record<string, unknown>
    const out: Record<string, unknown> = {}
    for (const k of Object.keys(o).sort()) {
      out[k] = sortKeys(o[k])
    }
    return out
  }
  return value
}

export function isDirty(baseline: unknown, current: unknown): boolean {
  return stableStringify(baseline) !== stableStringify(current)
}

/** 浅克隆 plain object / array，避免 baseline 被后续 mutation 污染 */
export function snapshotForm<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}
