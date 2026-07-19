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

// 将表单行转换为服务端 Points 格式（毫秒 precision，拒绝 NaN）
export function buildFormPoints(rows: FormRow[]): Record<string, unknown>[] {
  const points: Record<string, unknown>[] = []
  rows.forEach((row, rowIdx) => {
    const tags: Record<string, string> = {}
    for (const t of row.tags) {
      if (t.key.trim()) tags[t.key.trim()] = t.value
    }
    const fields: Record<string, Record<string, unknown>> = {}
    for (const f of row.fields) {
      if (!f.key.trim() || f.value === '') continue
      const ft = fieldTypes.find((t) => t.value === f.type)
      if (!ft) throw new Error(`第 ${rowIdx + 1} 行字段类型无效: ${f.type}`)
      const key = f.key.trim()
      switch (f.type) {
        case 'int': {
          if (!/^-?\d+$/.test(f.value.trim())) {
            throw new Error(`第 ${rowIdx + 1} 行字段 ${key} 不是合法整数`)
          }
          const intVal = Number(f.value)
          if (!Number.isSafeInteger(intVal)) {
            throw new Error(`第 ${rowIdx + 1} 行字段 ${key} 超出安全整数范围`)
          }
          fields[key] = { type: ft.goType, int64: intVal }
          break
        }
        case 'float': {
          const floatVal = Number(f.value)
          if (!Number.isFinite(floatVal)) {
            throw new Error(`第 ${rowIdx + 1} 行字段 ${key} 不是合法浮点数`)
          }
          fields[key] = { type: ft.goType, float64: floatVal }
          break
        }
        case 'bool':
          fields[key] = { type: ft.goType, bool: f.value === 'true' || f.value === '1' }
          break
        default:
          fields[key] = { type: ft.goType, string: f.value }
      }
    }
    if (!Object.keys(fields).length) {
      throw new Error(`第 ${rowIdx + 1} 行没有有效字段`)
    }
    let timestamp = Date.now()
    if (row.timestamp.trim()) {
      if (!/^-?\d+$/.test(row.timestamp.trim())) {
        throw new Error(`第 ${rowIdx + 1} 行时间戳必须是整数（毫秒）`)
      }
      timestamp = Number(row.timestamp)
      if (!Number.isSafeInteger(timestamp)) {
        throw new Error(`第 ${rowIdx + 1} 行时间戳超出安全整数范围，请使用毫秒`)
      }
    }
    points.push({
      measurement: row.measurement || 'data',
      tags,
      fields,
      timestamp,
      precision: 'ms',
    })
  })
  return points
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

export interface LineProtocolDiagnostics {
  points: Record<string, unknown>[]
  errors: string[]
  skippedLines: number
}

function parseFieldValue(val: string): Record<string, unknown> | null {
  if (val.endsWith('i')) {
    const raw = val.slice(0, -1)
    if (!/^-?\d+$/.test(raw)) return null
    const intVal = Number(raw)
    if (!Number.isSafeInteger(intVal)) return null
    return { type: 2, int64: intVal }
  }
  if (val === 'true' || val === 'false') {
    return { type: 4, bool: val === 'true' }
  }
  if (val.startsWith('"') && val.endsWith('"')) {
    return { type: 3, string: val.slice(1, -1) }
  }
  if (/^-?\d+\.\d+$/.test(val) || /^-?\d+e[+-]?\d+$/i.test(val)) {
    const floatVal = Number(val)
    if (!Number.isFinite(floatVal)) return null
    return { type: 1, float64: floatVal }
  }
  if (/^-?\d+$/.test(val)) {
    const intVal = Number(val)
    if (!Number.isSafeInteger(intVal)) return null
    return { type: 2, int64: intVal }
  }
  return { type: 3, string: val }
}

// 解析 InfluxDB line protocol 文本为 Points（带行级诊断）
export function parseLineProtocolDetailed(text: string): LineProtocolDiagnostics {
  const points: Record<string, unknown>[] = []
  const errors: string[] = []
  let skippedLines = 0
  const lines = text.split('\n')
  for (let i = 0; i < lines.length; i++) {
    const lineNo = i + 1
    const line = lines[i].trim()
    if (!line || line.startsWith('#')) {
      skippedLines++
      continue
    }
    const parts = parseLine(line)
    if (!parts) {
      errors.push(`第 ${lineNo} 行：格式非法`)
      continue
    }
    const tags: Record<string, string> = {}
    if (parts.tags) {
      for (const t of parts.tags.split(',')) {
        const eq = t.indexOf('=')
        if (eq <= 0) {
          errors.push(`第 ${lineNo} 行：标签无效 "${t}"`)
          continue
        }
        tags[t.slice(0, eq)] = t.slice(eq + 1)
      }
    }
    const fields: Record<string, Record<string, unknown>> = {}
    let fieldErrors = 0
    for (const f of parts.fields.split(',')) {
      if (!f) continue
      const eqIdx = f.indexOf('=')
      if (eqIdx < 0) {
        errors.push(`第 ${lineNo} 行：字段缺少 "=" (${f})`)
        fieldErrors++
        continue
      }
      const key = f.slice(0, eqIdx).trim()
      const val = f.slice(eqIdx + 1).trim()
      if (!key) {
        errors.push(`第 ${lineNo} 行：字段名为空`)
        fieldErrors++
        continue
      }
      const parsed = parseFieldValue(val)
      if (!parsed) {
        errors.push(`第 ${lineNo} 行：字段 ${key} 值非法 (${val})`)
        fieldErrors++
        continue
      }
      fields[key] = parsed
    }
    if (!Object.keys(fields).length) {
      errors.push(`第 ${lineNo} 行：无有效字段`)
      continue
    }
    let ts = parts.timestamp ? Number(parts.timestamp) : Date.now()
    let precision: 'ns' | 'ms' = 'ns'
    if (!parts.timestamp) {
      precision = 'ms'
    } else if (!Number.isSafeInteger(ts)) {
      errors.push(`第 ${lineNo} 行：时间戳超出 JS 安全整数 (${parts.timestamp})`)
      continue
    } else if (Math.abs(ts) < 1e15) {
      precision = 'ms'
    }
    points.push({
      measurement: parts.measurement,
      tags,
      fields,
      timestamp: ts,
      precision,
    })
    if (fieldErrors > 0) {
      // 部分字段失败已记录，点仍可写入
    }
  }
  return { points, errors, skippedLines }
}

// 兼容旧接口：仅返回 points；坏行会抛出汇总错误
export function parseLineProtocol(text: string): Record<string, unknown>[] {
  const { points, errors } = parseLineProtocolDetailed(text)
  if (!points.length && errors.length) {
    throw new Error(errors.slice(0, 5).join('; ') + (errors.length > 5 ? ` 等共 ${errors.length} 处` : ''))
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
    const fv = Number(valueStr)
    if (!Number.isFinite(fv)) continue
    fields.value = { type: 1, float64: fv }

    const tsMs = timestampStr ? Number(timestampStr) : Date.now()
    if (!Number.isFinite(Number(valueStr))) continue

    points.push({
      measurement: metricName,
      tags,
      fields,
      timestamp: Number.isFinite(tsMs) ? tsMs : Date.now(),
      precision: 'ms',
    })
  }
  return points
}