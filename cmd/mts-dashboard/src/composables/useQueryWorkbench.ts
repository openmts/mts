import { ref } from 'vue'
import { formatCaughtError } from '@/utils/apiError'
import { apiPost, apiPostNDJSONStream } from '@/api/client'
import {
  listDatabasesDetailed,
  listFields,
  listMeasurements,
  listRetentionPoliciesDetailed,
  listSeries,
  type FieldMeta,
  type SeriesMeta,
} from '@/api/meta'
import { capSeriesList, fieldNames, tagsToExpr } from '@/utils/seriesMeta'
import { makeFormErrorT } from '@/utils/formErrors'
import { buildQueryFromForm } from '@/utils/queryFormBuild'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { formatMessage } from '@/utils/formatMessage'
import type { QueryResultRow, QueryStatsData } from '@/api/types'

export type { QueryResultRow, QueryStatsData }

export type QueryMode = 'rows' | 'columns' | 'explain' | 'stream-row' | 'stream-column'

export function useQueryWorkbench() {
  const databases = ref<string[]>([])
  const measurements = ref<string[]>([])
  const retentionPolicies = ref<string[]>([])
  const measurementsLoading = ref(false)
  const fieldOptions = ref<string[]>([])
  const seriesOptions = ref<SeriesMeta[]>([])
  const seriesTotal = ref(0)
  const seriesTruncated = ref(false)
  const seriesLoading = ref(false)
  const seriesError = ref('')
  const SERIES_CAP = 200
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
    // 谓词 DSL，见 parsePredicates
    predicates: '',
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
    fieldOptions.value = []
    seriesOptions.value = []
    seriesTotal.value = 0
    seriesTruncated.value = false
    seriesError.value = ''
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
        const rpResult = await listRetentionPoliciesDetailed(db)
        retentionPolicies.value = rpResult.policies.map((p) => p.name)
        if (retentionPolicies.value.length && !retentionPolicies.value.includes(queryForm.value.retention_policy)) {
          queryForm.value.retention_policy = retentionPolicies.value[0]
        }
        // partial: 库列表可能 admin，RP 走 data
        if (rpResult.source === 'data' && metaSource.value === 'admin') {
          // keep admin for db list
        } else if (rpResult.source === 'manual' && metaSource.value === 'admin') {
          metaSource.value = 'partial'
        }
      } catch {
        // RP 列表失败时允许手填
        retentionPolicies.value = []
      }
    } finally {
      measurementsLoading.value = false
    }
  }

  async function loadMeasurementMeta(db: string, measurement: string) {
    fieldOptions.value = []
    seriesOptions.value = []
    seriesTotal.value = 0
    seriesTruncated.value = false
    seriesError.value = ''
    if (!db.trim() || !measurement.trim()) return
    seriesLoading.value = true
    try {
      const [fields, series] = await Promise.all([
        listFields(db, measurement).catch(() => [] as FieldMeta[]),
        listSeries(db, measurement).catch((e) => {
          seriesError.value = formatCaughtError(e)
          return [] as SeriesMeta[]
        }),
      ])
      fieldOptions.value = fieldNames(fields)
      const capped = capSeriesList(series, SERIES_CAP)
      seriesOptions.value = capped.items
      seriesTotal.value = capped.total
      seriesTruncated.value = capped.truncated
    } finally {
      seriesLoading.value = false
    }
  }

  function applySeriesTags(s: SeriesMeta) {
    queryForm.value.tags = tagsToExpr(s.tags)
  }

  const { t: tMsg } = useI18n()
  const formT = () =>
    makeFormErrorT({
      queryErrAggFormat: tMsg.value('queryErrAggFormat' as MessageKey),
      queryErrAggEmpty: tMsg.value('queryErrAggEmpty' as MessageKey),
      queryErrTagFormat: tMsg.value('queryErrTagFormat' as MessageKey),
      queryErrTagKeyEmpty: tMsg.value('queryErrTagKeyEmpty' as MessageKey),
      queryErrStartTime: tMsg.value('queryErrStartTime' as MessageKey),
      queryErrEndTime: tMsg.value('queryErrEndTime' as MessageKey),
      queryErrOffset: tMsg.value('queryErrOffset' as MessageKey),
      queryErrLimit: tMsg.value('queryErrLimit' as MessageKey),
      queryErrPredFormat: tMsg.value('queryErrPredFormat' as MessageKey),
      queryErrPredKind: tMsg.value('queryErrPredKind' as MessageKey),
      queryErrPredName: tMsg.value('queryErrPredName' as MessageKey),
    })

  function buildQuery(): Record<string, unknown> {
    return buildQueryFromForm(queryForm.value, formT())
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
        rawOutput.value =
          preview.join('\n') +
          (previewOnly
            ? '\n' +
              formatMessage(tMsg.value('queryStreamPreviewFooter' as MessageKey), {
                lines,
                limit: preview.length,
              })
            : '')
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
    fieldOptions,
    seriesOptions,
    seriesTotal,
    seriesTruncated,
    seriesLoading,
    seriesError,
    loadMeasurementMeta,
    applySeriesTags,
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
