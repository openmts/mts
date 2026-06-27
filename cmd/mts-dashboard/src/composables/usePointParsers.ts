// 数据点解析相关类型与函数，从 WritePage 抽取以控制单文件行数

export interface TagRow { key: string; value: string }
export interface FieldRow { key: string; value: string; type: string }
export interface FormRow {
  measurement: string
  tags: TagRow[]
  fields: FieldRow[]
  timestamp: string
}

export interface FieldTypeOption {
  value: string
  label: string
  goType: number
}

// 字段类型选项，goType 对应服务端字段类型枚举
export const fieldTypes: FieldTypeOption[] = [
  { value: 'float', label: 'Float', goType: 1 },
  { value: 'int', label: 'Int', goType: 2 },
  { value: 'string', label: 'String', goType: 3 },
  { value: 'bool', label: 'Bool', goType: 4 },
]

// 将表单行转换为服务端 Points 格式
export function buildFormPoints(rows: FormRow[]): Record<string, unknown>[] {
  return rows.map((row) => {
    const tags: Record<string, string> = {}
    for (const t of row.tags) { if (t.key.trim()) tags[t.key.trim()] = t.value }
    const fields: Record<string, Record<string, unknown>> = {}
    for (const f of row.fields) {
      if (!f.key.trim() || f.value === '') continue
      const ft = fieldTypes.find((t) => t.value === f.type)
      switch (f.type) {
        case 'int': fields[f.key.trim()] = { type: ft!.goType, int64: parseInt(f.value) }; break
        case 'float': fields[f.key.trim()] = { type: ft!.goType, float64: parseFloat(f.value) }; break
        case 'bool': fields[f.key.trim()] = { type: ft!.goType, bool: f.value === 'true' }; break
        default: fields[f.key.trim()] = { type: ft!.goType, string: f.value }
      }
    }
    return {
      measurement: row.measurement || 'data',
      tags,
      fields,
      timestamp: parseInt(row.timestamp) || Date.now() * 1e6,
    }
  })
}

interface ParsedLine {
  measurement: string
  tags: string
  fields: string
  timestamp?: string
}

// 拆分单行 line protocol 为 measurement/tags/fields/timestamp
export function parseLine(line: string): ParsedLine | null {
  let remaining = line
  let measurement: string
  let tags = ''
  let fields: string
  let timestamp: string | undefined

  const firstComma = remaining.indexOf(',')
  const firstSpace = remaining.indexOf(' ')
  if (firstSpace < 0) return null

  if (firstComma >= 0 && firstComma < firstSpace) {
    measurement = remaining.slice(0, firstComma)
    tags = remaining.slice(firstComma + 1, firstSpace)
  } else {
    measurement = remaining.slice(0, firstSpace)
  }
  remaining = remaining.slice(firstSpace + 1)

  const lastSpace = remaining.lastIndexOf(' ')
  if (lastSpace >= 0) {
    const after = remaining.slice(lastSpace + 1)
    if (/^\d+$/.test(after)) {
      fields = remaining.slice(0, lastSpace)
      timestamp = after
    } else {
      fields = remaining
    }
  } else {
    fields = remaining
  }
  return { measurement, tags, fields, timestamp }
}

// 解析 InfluxDB line protocol 文本为 Points
export function parseLineProtocol(text: string): Record<string, unknown>[] {
  const points: Record<string, unknown>[] = []
  for (const rawLine of text.split('\n')) {
    const line = rawLine.trim()
    if (!line || line.startsWith('#')) continue
    const parts = parseLine(line)
    if (!parts) continue
    const tags: Record<string, string> = {}
    if (parts.tags) {
      for (const t of parts.tags.split(',')) {
        const [k, v] = t.split('=')
        if (k) tags[k] = v ?? ''
      }
    }
    const fields: Record<string, Record<string, unknown>> = {}
    for (const f of parts.fields.split(',')) {
      const eqIdx = f.indexOf('=')
      if (eqIdx < 0) continue
      const key = f.slice(0, eqIdx).trim()
      const val = f.slice(eqIdx + 1).trim()
      if (!key) continue
      if (val.endsWith('i')) {
        const intVal = parseInt(val.slice(0, -1))
        if (isNaN(intVal)) continue
        fields[key] = { type: 2, int64: intVal }
      } else if (val === 'true' || val === 'false') {
        fields[key] = { type: 4, bool: val === 'true' }
      } else if (val.startsWith('"') && val.endsWith('"')) {
        fields[key] = { type: 3, string: val.slice(1, -1) }
      } else if (/^-?\d+\.\d+$/.test(val) || /^-?\d+e[+-]?\d+$/i.test(val)) {
        const floatVal = parseFloat(val)
        if (isNaN(floatVal)) continue
        fields[key] = { type: 1, float64: floatVal }
      } else if (/^-?\d+$/.test(val)) {
        const intVal = parseInt(val)
        if (isNaN(intVal)) continue
        fields[key] = { type: 2, int64: intVal }
      } else {
        fields[key] = { type: 3, string: val }
      }
    }
    points.push({
      measurement: parts.measurement,
      tags,
      fields,
      timestamp: parseInt(parts.timestamp ?? String(Date.now() * 1e6)),
    })
  }
  return points
}

// 解析 Prometheus exposition 文本为 Points
export function parsePrometheusText(text: string): Record<string, unknown>[] {
  const points: Record<string, unknown>[] = []
  for (const rawLine of text.split('\n')) {
    const line = rawLine.trim()
    if (!line || line.startsWith('#')) continue

    // 匹配: metric_name{labels} value [timestamp]
    const match = line.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*)\s*(?:\{([^}]*)\})?\s+(-?\d+(?:\.\d+)?(?:e[+-]?\d+)?)\s*(\d+)?$/)
    if (!match) continue

    const [, metricName, labelsStr, valueStr, timestampStr] = match

    const tags: Record<string, string> = {}
    if (labelsStr) {
      // 解析 "key1=\"v1\",key2=\"v2\"" 形式的标签
      let i = 0
      while (i < labelsStr.length) {
        // 跳过空白和逗号
        while (i < labelsStr.length && (labelsStr[i] === ' ' || labelsStr[i] === ',')) i++
        if (i >= labelsStr.length) break
        // 读取 key
        const eqIdx = labelsStr.indexOf('=', i)
        if (eqIdx < 0) break
        const key = labelsStr.slice(i, eqIdx).trim()
        i = eqIdx + 1
        // 跳过等号两侧空白
        while (i < labelsStr.length && labelsStr[i] === ' ') i++
        // 读取引号内的 value
        if (labelsStr[i] !== '"') break
        i++
        let val = ''
        while (i < labelsStr.length && labelsStr[i] !== '"') {
          if (labelsStr[i] === '\\') i++
          if (i < labelsStr.length) val += labelsStr[i]
          i++
        }
        i++ // 跳过后引号
        if (key) tags[key] = val
      }
    }

    const fields: Record<string, Record<string, unknown>> = {}
    fields.value = { type: 1, float64: parseFloat(valueStr) }

    const tsMs = timestampStr ? parseInt(timestampStr) : Date.now()
    // Prometheus 时间戳通常是毫秒级，转为纳秒
    const tsNs = timestampStr && tsMs < 1e15 ? tsMs * 1e6 : tsMs

    points.push({
      measurement: metricName,
      tags,
      fields,
      timestamp: tsNs,
    })
  }
  return points
}