import { ref } from 'vue'
import { formatCaughtError } from '@/utils/apiError'
import { apiPost, apiPostNDJSONStream } from '@/api/client'
import { listDatabasesDetailed, listMeasurements, listRetentionPolicies } from '@/api/meta'
import { parseTimeInt } from '@/utils/time'
import { parseHumanDurationToNs } from '@/utils/duration'
import type { QueryResultRow, QueryStatsData } from '@/api/types'

export type { QueryResultRow, QueryStatsData }

export type QueryMode = 'rows' | 'columns' | 'explain' | 'stream-row' | 'stream-column'

export function useQueryWorkbench() {
  const databases = ref<string[]>([])
  const measurements = ref<string[]>([])
  const retentionPolicies = ref<string[]>([])
  const measurementsLoading = ref(false)
  const metaSource = ref<'admin' | 'manual' | 'partial'>('admin')
  const metaHint = ref('')
  const queryForm = ref({
    database: '',
    retention_policy: 'autogen',
    measurement: '',
    start_time: '',
    end_time: '',
    fields: '',
    tags: '', // key=value,key2=value2
    order: 'asc' as 'asc' | 'desc' | '',
    offset: '',
    limit: '100',
    // 聚合：func:field 逗号分隔，如 mean:usage,max:usage
    aggregates: '',
    // 窗口：人类可读 duration，如 1m/5m；序列化为纳秒
    window: '',
    // group by tags：逗号分隔
    group_tags: '',
  })
  const queryMode = ref<QueryMode>('rows')
  const rows = ref<QueryResultRow[]>([])
  const queryStats = ref<QueryStatsData | null>(null)
  const rawOutput = ref('')
  const columnSeries = ref<unknown[]>([])
  const streamMeta = ref({ lines: 0, records: 0, errors: 0, previewOnly: false, previewLimit: 200 })
  const actionError = ref('')
  const loading = ref(false)
  let queryAbort: AbortController | null = null
  let requestSeq = 0

  async function loadDatabases() {
    const result = await listDatabasesDetailed()
    databases.value = result.names
    metaSource.value = result.source
    metaHint.value = result.error || ''
    if (databases.value.length && !queryForm.value.database) {
      queryForm.value.database = databases.value[0]
    }
  }

  async function loadDbChildren(db: string) {
    measurements.value = []
    retentionPolicies.value = []
    if (!queryForm.value.measurement) queryForm.value.measurement = ''
    if (!db) return
    measurementsLoading.value = true
    try {
      const meas = await listMeasurements(db)
      measurements.value = meas
      if (meas.length && !queryForm.value.measurement) {
        queryForm.value.measurement = meas[0]
      }
      try {
        const rps = await listRetentionPolicies(db)
        retentionPolicies.value = rps.map((p) => p.name)
        if (retentionPolicies.value.length && !retentionPolicies.value.includes(queryForm.value.retention_policy)) {
          queryForm.value.retention_policy = retentionPolicies.value[0]
        }
      } catch {
        // RP 列表失败时允许手填
        retentionPolicies.value = []
      }
    } finally {
      measurementsLoading.value = false
    }
  }

  function parseAggregates(text: string): { function: string; field: string }[] {
    const out: { function: string; field: string }[] = []
    for (const part of text.split(',')) {
      const s = part.trim()
      if (!s) continue
      const colon = s.indexOf(':')
      if (colon <= 0) throw new Error(`聚合格式无效: ${s}（需要 func:field，如 mean:usage）`)
      const fn = s.slice(0, colon).trim().toLowerCase()
      const field = s.slice(colon + 1).trim()
      if (!fn || !field) throw new Error(`聚合格式无效: ${s}`)
      out.push({ function: fn, field })
    }
    return out
  }

  function parseTags(text: string): Record<string, string> {
    const tags: Record<string, string> = {}
    for (const part of text.split(',')) {
      const s = part.trim()
      if (!s) continue
      const eq = s.indexOf('=')
      if (eq <= 0) throw new Error(`tag 格式无效: ${s}（需要 key=value）`)
      const k = s.slice(0, eq).trim()
      const v = s.slice(eq + 1).trim()
      if (!k) throw new Error(`tag key 为空: ${s}`)
      tags[k] = v
    }
    return tags
  }

  function buildQuery(): Record<string, unknown> {
    const query: Record<string, unknown> = {
      precision: 'ms',
    }
    if (queryForm.value.database) query.database = queryForm.value.database
    if (queryForm.value.retention_policy) query.retention_policy = queryForm.value.retention_policy
    if (queryForm.value.measurement) query.measurement = queryForm.value.measurement
    if (queryForm.value.start_time) {
      const v = parseTimeInt(queryForm.value.start_time)
      if (v === null) throw new Error('start_time 必须是安全整数（推荐毫秒）')
      query.start_time = v
    }
    if (queryForm.value.end_time) {
      const v = parseTimeInt(queryForm.value.end_time)
      if (v === null) throw new Error('end_time 必须是安全整数（推荐毫秒）')
      query.end_time = v
    }
    if (queryForm.value.fields) {
      query.fields = queryForm.value.fields.split(',').map((s) => s.trim()).filter(Boolean)
    }
    if (queryForm.value.tags.trim()) {
      query.tags = parseTags(queryForm.value.tags)
    }
    if (queryForm.value.order === 'asc' || queryForm.value.order === 'desc') {
      // QueryOrder: by=1(time), direction=1(asc)/2(desc)
      query.order = {
        by: 1,
        direction: queryForm.value.order === 'desc' ? 2 : 1,
      }
    }
    if (queryForm.value.offset) {
      const off = parseTimeInt(queryForm.value.offset)
      if (off === null || off < 0) throw new Error('offset 必须是非负整数')
      query.offset = off
    }
    if (queryForm.value.limit) {
      const lim = parseTimeInt(queryForm.value.limit)
      if (lim === null || lim <= 0) throw new Error('limit 必须是正整数')
      query.limit = lim
    }
    if (queryForm.value.aggregates.trim()) {
      query.aggregates = parseAggregates(queryForm.value.aggregates)
    }
    if (queryForm.value.window.trim()) {
      const ns = parseHumanDurationToNs(queryForm.value.window)
      query.window = ns
      const groupTags = queryForm.value.group_tags
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
      query.group = { tags: groupTags, window: ns }
    } else if (queryForm.value.group_tags.trim()) {
      query.group = {
        tags: queryForm.value.group_tags.split(',').map((s) => s.trim()).filter(Boolean),
      }
    }
    return query
  }

  function cancelQuery() {
    if (queryAbort) {
      queryAbort.abort()
      queryAbort = null
    }
    requestSeq += 1
  }

  function beginRequest(): { signal: AbortSignal; seq: number } {
    cancelQuery()
    queryAbort = new AbortController()
    const seq = ++requestSeq
    return { signal: queryAbort.signal, seq }
  }

  async function executeQuery() {
    actionError.value = ''
    loading.value = true
    rows.value = []
    columnSeries.value = []
    queryStats.value = null
    rawOutput.value = ''
    streamMeta.value = { lines: 0, records: 0, errors: 0, previewOnly: false, previewLimit: 200 }
    const { signal, seq } = beginRequest()
    try {
      const query = buildQuery()
      if (queryMode.value === 'rows') {
        const data = await apiPost<{ rows: QueryResultRow[]; stats?: QueryStatsData }>(
          '/api/v1/data/query/rows',
          { query },
          { signal },
        )
        if (seq !== requestSeq) return
        rows.value = data.rows ?? []
        if (data.stats) queryStats.value = data.stats
      } else if (queryMode.value === 'explain') {
        const data = await apiPost<{
          result: {
            columns: unknown[]
            explain: Record<string, unknown>
            stats: QueryStatsData
          }
        }>('/api/v1/data/query/explain', { query }, { signal })
        if (seq !== requestSeq) return
        queryStats.value = data.result?.stats ?? null
        const payload = {
          explain: data.result?.explain ?? null,
          stats: data.result?.stats ?? null,
          columns: data.result?.columns ?? [],
        }
        rawOutput.value = JSON.stringify(payload, null, 2)
      } else if (queryMode.value === 'columns') {
        const data = await apiPost<{ columns: unknown[]; stats?: QueryStatsData }>(
          '/api/v1/data/query/columns',
          { query },
          { signal },
        )
        if (seq !== requestSeq) return
        columnSeries.value = data.columns ?? []
        rawOutput.value = JSON.stringify(data.columns, null, 2)
        if (data.stats) queryStats.value = data.stats
      } else {
        const format = queryMode.value === 'stream-column' ? 'column' : 'row'
        const preview: string[] = []
        let lines = 0
        let records = 0
        let errors = 0
        let endStats: QueryStatsData | null = null
        let streamError = ''
        await apiPostNDJSONStream(
          '/api/v1/data/query/stream',
          { query, format },
          (line, rec, parseError) => {
            if (seq !== requestSeq) return
            lines += 1
            if (preview.length < 200) preview.push(line)
            if (parseError || !rec || typeof rec !== 'object') {
              errors += 1
              return
            }
            const obj = rec as {
              type?: string
              stats?: QueryStatsData
              error?: { message?: string }
            }
            const type = obj.type ?? ''
            if (type === 'row' || type === 'column') records += 1
            else if (type === 'end' && obj.stats) endStats = obj.stats
            else if (type === 'error') {
              errors += 1
              streamError = obj.error?.message || streamError || formatCaughtError({ code: 'internal', message: 'stream' })
            }
          },
          { signal },
        )
        if (seq !== requestSeq) return
        const previewOnly = lines > preview.length
        streamMeta.value = { lines, records, errors, previewOnly, previewLimit: 200 }
        rawOutput.value = preview.join('\n') + (previewOnly ? `\n… 共 ${lines} 行，仅预览前 ${preview.length} 行（复制也仅含预览）` : '')
        if (endStats) queryStats.value = endStats
        if (streamError) actionError.value = streamError
      }
    } catch (e) {
      if (seq !== requestSeq) return
      actionError.value = formatCaughtError(e)
    } finally {
      if (seq === requestSeq) {
        loading.value = false
        queryAbort = null
      }
    }
  }

  function resultTextForCopy(): string {
    if (rawOutput.value) return rawOutput.value
    if (rows.value.length) return JSON.stringify(rows.value, null, 2)
    return ''
  }

  return {
    databases,
    measurements,
    retentionPolicies,
    measurementsLoading,
    metaSource,
    metaHint,
    queryForm,
    queryMode,
    rows,
    columnSeries,
    queryStats,
    rawOutput,
    streamMeta,
    actionError,
    loading,
    loadDatabases,
    loadDbChildren,
    executeQuery,
    cancelQuery,
    resultTextForCopy,
    buildQuery,
  }
}
