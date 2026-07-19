import { ref } from 'vue'
import { apiPost, apiPostNDJSONStream, APIClientError } from '@/api/client'
import { listDatabases, listMeasurements, listRetentionPolicies } from '@/api/meta'
import { parseTimeInt } from '@/utils/time'

export interface QueryResultRow {
  series_id: number
  measurement: string
  tags: Record<string, string>
  timestamp: number
  fields: Record<string, unknown>
}

export interface QueryStatsData {
  candidate_shards: number
  shards_scanned: number
  shards_skipped: number
  parts_scanned: number
  parts_skipped: number
  samples_read: number
  samples_returned: number
  duration_nanos: number
  errors: number
}

export type QueryMode = 'rows' | 'columns' | 'explain' | 'stream-row' | 'stream-column'

export function useQueryWorkbench() {
  const databases = ref<string[]>([])
  const measurements = ref<string[]>([])
  const retentionPolicies = ref<string[]>([])
  const measurementsLoading = ref(false)
  const queryForm = ref({
    database: '',
    retention_policy: 'autogen',
    measurement: '',
    start_time: '',
    end_time: '',
    fields: '',
    limit: '100',
  })
  const queryMode = ref<QueryMode>('rows')
  const rows = ref<QueryResultRow[]>([])
  const queryStats = ref<QueryStatsData | null>(null)
  const rawOutput = ref('')
  const streamMeta = ref({ lines: 0, records: 0, errors: 0, previewOnly: false, previewLimit: 200 })
  const actionError = ref('')
  const loading = ref(false)
  let queryAbort: AbortController | null = null
  let requestSeq = 0

  async function loadDatabases() {
    databases.value = await listDatabases()
    if (databases.value.length && !queryForm.value.database) {
      queryForm.value.database = databases.value[0]
    }
  }

  async function loadDbChildren(db: string) {
    measurements.value = []
    retentionPolicies.value = []
    queryForm.value.measurement = ''
    queryForm.value.retention_policy = 'autogen'
    if (!db) return
    measurementsLoading.value = true
    try {
      const [meas, rps] = await Promise.all([
        listMeasurements(db),
        listRetentionPolicies(db),
      ])
      measurements.value = meas
      retentionPolicies.value = rps.map((p) => p.name)
      if (measurements.value.length) {
        queryForm.value.measurement = measurements.value[0]
      }
    } finally {
      measurementsLoading.value = false
    }
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
    if (queryForm.value.limit) {
      const lim = parseTimeInt(queryForm.value.limit)
      if (lim === null || lim <= 0) throw new Error('limit 必须是正整数')
      query.limit = lim
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
              streamError = obj.error?.message || streamError || '流式查询错误'
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
      if (e instanceof APIClientError && (e.code === 'canceled' || e.status === 499)) {
        actionError.value = '查询已取消'
      } else {
        actionError.value = e instanceof Error ? e.message : '查询失败'
      }
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
    queryForm,
    queryMode,
    rows,
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
