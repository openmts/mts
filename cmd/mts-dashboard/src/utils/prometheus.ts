/** 轻量 Prometheus text exposition 解析（仅用于 Dashboard 浏览，不求完整兼容） */

export interface PrometheusSample {
  name: string
  labels: Record<string, string>
  value: number
  raw: string
}

export interface PrometheusFamily {
  name: string
  help: string
  type: string
  samples: PrometheusSample[]
}

const SAMPLE_RE = /^([a-zA-Z_:][a-zA-Z0-9_:]*)(\{([^}]*)\})?\s+([^\s]+)(?:\s+\d+)?\s*$/

function parseLabels(raw: string): Record<string, string> {
  const out: Record<string, string> = {}
  if (!raw.trim()) return out
  // key="value",key2="v2"
  const re = /([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*"((?:\\.|[^"\\])*)"/g
  let m: RegExpExecArray | null
  while ((m = re.exec(raw)) !== null) {
    out[m[1]] = m[2].replace(/\\n/g, '\n').replace(/\\"/g, '"').replace(/\\\\/g, '\\')
  }
  return out
}

export function parsePrometheusText(text: string): PrometheusFamily[] {
  const families = new Map<string, PrometheusFamily>()
  let pendingHelp = ''
  let pendingType = ''
  let pendingName = ''

  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim()
    if (!trimmed) continue
    if (trimmed.startsWith('#')) {
      const help = trimmed.match(/^#\s+HELP\s+(\S+)\s+(.*)$/)
      if (help) {
        pendingName = help[1]
        pendingHelp = help[2]
        continue
      }
      const typ = trimmed.match(/^#\s+TYPE\s+(\S+)\s+(\S+)/)
      if (typ) {
        pendingName = typ[1]
        pendingType = typ[2]
        continue
      }
      continue
    }
    const m = trimmed.match(SAMPLE_RE)
    if (!m) continue
    const name = m[1]
    const labels = parseLabels(m[3] || '')
    const value = Number(m[4])
    if (!Number.isFinite(value)) continue
    let fam = families.get(name)
    if (!fam) {
      fam = {
        name,
        help: name === pendingName ? pendingHelp : '',
        type: name === pendingName ? pendingType : '',
        samples: [],
      }
      families.set(name, fam)
    } else {
      if (!fam.help && name === pendingName && pendingHelp) fam.help = pendingHelp
      if (!fam.type && name === pendingName && pendingType) fam.type = pendingType
    }
    fam.samples.push({ name, labels, value, raw: trimmed })
  }
  return Array.from(families.values()).sort((a, b) => a.name.localeCompare(b.name))
}

export function filterPrometheusFamilies(
  families: PrometheusFamily[],
  query: string,
): PrometheusFamily[] {
  const q = query.trim().toLowerCase()
  if (!q) return families
  return families
    .map((f) => {
      const nameHit = f.name.toLowerCase().includes(q)
      const helpHit = f.help.toLowerCase().includes(q)
      const samples = f.samples.filter((s) => {
        if (nameHit || helpHit) return true
        if (s.raw.toLowerCase().includes(q)) return true
        return Object.entries(s.labels).some(
          ([k, v]) => k.toLowerCase().includes(q) || v.toLowerCase().includes(q),
        )
      })
      if (!samples.length && !nameHit && !helpHit) return null
      return { ...f, samples: samples.length ? samples : f.samples }
    })
    .filter((x): x is PrometheusFamily => x !== null)
}

export function formatSampleLabels(labels: Record<string, string>): string {
  const keys = Object.keys(labels).sort()
  if (!keys.length) return ''
  return keys.map((k) => `${k}="${labels[k]}"`).join(', ')
}

export function summarizeFamilies(families: PrometheusFamily[]): {
  families: number
  samples: number
} {
  return {
    families: families.length,
    samples: families.reduce((n, f) => n + f.samples.length, 0),
  }
}
