import { ref } from 'vue'
import { formatCaughtError, resolveCaughtErrorCode } from '@/utils/apiError'
import { apiPost, apiPostNDJSONStream } from '@/api/client'
import {
  listDatabasesDetailed,
  listFields,
  listMeasurements,
  listRetentionPoliciesDetailed,
  listSeriesDetailed,
  type FieldMeta,
  type MetaLoadSource,
  type SeriesMeta,
} from '@/api/meta'
import { fieldNames, tagsToExpr } from '@/utils/seriesMeta'
import { makeFormErrorT } from '@/utils/formErrors'
import { buildQueryFromForm, parseTags } from '@/utils/queryFormBuild'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { formatMessage } from '@/utils/formatMessage'
import { fetchEngineQueryStats } from '@/api/queryMeta'
import type { QueryResultRow, QueryStatsData } from '@/api/types'
import { hasQueryResultSnapshot } from '@/utils/querySnapshot'

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
  const seriesOffset = ref(0)
  const seriesHasMore = ref(false)
  const SERIES_CAP = 200
  const metaSource = ref<MetaLoadSource>('admin')
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
  const engineStatsSource = ref<'query' | 'engine' | ''>('')
  const engineStatsLoading = ref(false)
  const engineStatsError = ref('')
  const engineStatsAt = ref<number | null>(null)
  const rawOutput = ref('')
  const columnSeries = ref<unknown[]>([])
  const streamMeta = ref({ lines: 0, records: 0, errors: 0, previewOnly: false, previewLimit: 200 })
  const actionError = ref('')
  const lastQueryErrorCode = ref('')
  const loading = ref(false)
  const queryStartedAt = ref<number | null>(null)
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
    seriesOffset.value = 0
    seriesHasMore.value = false
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

  function parseTagsSafe(text: string): { tags?: Record<string, string>; error?: string } {
    const raw = text.trim()
    if (!raw) return {}
    try {
      return { tags: parseTags(raw, formT()) }
    } catch (e) {
      return { error: formatCaughtError(e) }
    }
  }

  function seriesLabelKey(s: SeriesMeta): string {
    const tags = s.tags || {}
    return `${s.measurement || ''}|${Object.keys(tags).sort().map((k) => `${k}=${tags[k]}`).join(',')}`
  }

  function applySeriesPage(
    result: { series: SeriesMeta[]; total: number; truncated: boolean; offset?: number },
    append: boolean,
  ) {
    if (append) {
      const seen = new Set(seriesOptions.value.map((s) => String(s.id ?? seriesLabelKey(s))))
      for (const s of result.series) {
        const key = String(s.id ?? seriesLabelKey(s))
        if (!seen.has(key)) {
          seriesOptions.value.push(s)
          seen.add(key)
        }
      }
    } else {
      seriesOptions.value = result.series
    }
    seriesTotal.value = result.total
    seriesTruncated.value = result.truncated
    const base = result.offset ?? (append ? seriesOffset.value : 0)
    const nextOffset = base + result.series.length
    seriesOffset.value = nextOffset
    seriesHasMore.value = nextOffset < result.total
  }

  async function loadMeasurementMeta(db: string, measurement: string, opts?: { useFormTags?: boolean }) {
    fieldOptions.value = []
    seriesOptions.value = []
    seriesTotal.value = 0
    seriesTruncated.value = false
    seriesOffset.value = 0
    seriesHasMore.value = false
    seriesError.value = ''
    if (!db.trim() || !measurement.trim()) return
    seriesLoading.value = true
    try {
      let tagFilter: Record<string, string> | undefined
      if (opts?.useFormTags) {
        const parsed = parseTagsSafe(queryForm.value.tags)
        if (parsed.error) {
          seriesError.value = parsed.error
        } else {
          tagFilter = parsed.tags
        }
      }
      const [fields, seriesResult] = await Promise.all([
        listFields(db, measurement).catch(() => [] as FieldMeta[]),
        listSeriesDetailed(db, measurement, { tags: tagFilter, limit: SERIES_CAP, offset: 0 }).catch((e) => {
          seriesError.value = seriesError.value || formatCaughtError(e)
          return { series: [] as SeriesMeta[], total: 0, truncated: false, limit: SERIES_CAP, offset: 0 }
        }),
      ])
      fieldOptions.value = fieldNames(fields)
      applySeriesPage(seriesResult, false)
    } finally {
      seriesLoading.value = false
    }
  }

  /** 用当前表单 tags 向服务端重新过滤 series */
  async function refreshSeriesWithTags() {
    const db = queryForm.value.database
    const measurement = queryForm.value.measurement
    if (!db.trim() || !measurement.trim()) return
    seriesLoading.value = true
    seriesError.value = ''
    try {
      const parsed = parseTagsSafe(queryForm.value.tags)
      if (parsed.error) {
        seriesError.value = parsed.error
        return
      }
      seriesOffset.value = 0
      const seriesResult = await listSeriesDetailed(db, measurement, {
        tags: parsed.tags,
        limit: SERIES_CAP,
        offset: 0,
      })
      applySeriesPage(seriesResult, false)
    } catch (e) {
      seriesError.value = formatCaughtError(e)
    } finally {
      seriesLoading.value = false
    }
  }

  async function loadMoreSeries(opts?: { q?: string }) {
    const db = queryForm.value.database
    const measurement = queryForm.value.measurement
    if (!db.trim() || !measurement.trim() || !seriesHasMore.value || seriesLoading.value) return
    seriesLoading.value = true
    seriesError.value = ''
    try {
      const parsed = parseTagsSafe(queryForm.value.tags)
      if (parsed.error) {
        seriesError.value = parsed.error
        return
      }
      const seriesResult = await listSeriesDetailed(db, measurement, {
        tags: parsed.tags,
        limit: SERIES_CAP,
        offset: seriesOffset.value,
        q: opts?.q,
      })
      applySeriesPage(seriesResult, true)
    } catch (e) {
      seriesError.value = formatCaughtError(e)
    } finally {
      seriesLoading.value = false
    }
  }

  async function refreshSeriesWithServerQuery(q: string) {
    const db = queryForm.value.database
    const measurement = queryForm.value.measurement
    if (!db.trim() || !measurement.trim()) return
    seriesLoading.value = true
    seriesError.value = ''
    try {
      const parsed = parseTagsSafe(queryForm.value.tags)
      if (parsed.error) {
        seriesError.value = parsed.error
        return
      }
      seriesOffset.value = 0
      const seriesResult = await listSeriesDetailed(db, measurement, {
        tags: parsed.tags,
        limit: SERIES_CAP,
        offset: 0,
        q: q.trim() || undefined,
      })
      applySeriesPage(seriesResult, false)
    } catch (e) {
      seriesError.value = formatCaughtError(e)
    } finally {
      seriesLoading.value = false
    }
  }

  function applySeriesTags(s: SeriesMeta) {
    queryForm.value.tags = tagsToExpr(s.tags)
  }

  function buildQuery(): Record<string, unknown> {
    return buildQueryFromForm(queryForm.value, formT())
  }


  /** 仅中止在途请求（不改 seq） */
  function abortInFlight() {
    if (queryAbort) {
      queryAbort.abort()
      queryAbort = null
    }
  }

  /**
   * 用户取消查询：abort 后由 catch(isCanceled) 写文案、finally 清 loading。
   * 不推进 requestSeq，避免 finally 跳过导致 loading 卡住。
   * 卸载时同样调用：seq 仍匹配，loading 可被 finally 清掉。
   */
  function cancelQuery() {
    abortInFlight()
  }

  function beginRequest(): { signal: AbortSignal; seq: number } {
    // 新查询替换旧请求：先 abort 再推进 seq，旧请求 catch/finally 不再改 UI
    abortInFlight()
    queryAbort = new AbortController()
    const seq = ++requestSeq
    return { signal: queryAbort.signal, seq }
  }

  function hasQuerySnapshot(): boolean {
    return hasQueryResultSnapshot({
      rows: rows.value.length,
      columns: columnSeries.value.length,
      rawOutput: rawOutput.value,
      stats: !!queryStats.value,
    })
  }

  function clearQuerySnapshot() {
    rows.value = []
    columnSeries.value = []
    queryStats.value = null
    engineStatsSource.value = 'query'
    rawOutput.value = ''
    streamMeta.value = { lines: 0, records: 0, errors: 0, previewOnly: false, previewLimit: 200 }
  }

  async function executeQuery() {
    actionError.value = ''
    lastQueryErrorCode.value = ''
    engineStatsError.value = ''
    loading.value = true
    queryStartedAt.value = Date.now()
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
        columnSeries.value = []
        rawOutput.value = ''
        streamMeta.value = { lines: 0, records: 0, errors: 0, previewOnly: false, previewLimit: 200 }
        queryStats.value = data.stats ?? null
        engineStatsSource.value = 'query'
        actionError.value = ''
        lastQueryErrorCode.value = ''
      } else if (queryMode.value === 'explain') {
        const data = await apiPost<{
          result: {
            columns: unknown[]
            explain: Record<string, unknown>
            stats: QueryStatsData
          }
        }>('/api/v1/data/query/explain', { query }, { signal })
        if (seq !== requestSeq) return
        rows.value = []
        columnSeries.value = []
        queryStats.value = data.result?.stats ?? null
        engineStatsSource.value = 'query'
        const payload = {
          explain: data.result?.explain ?? null,
          stats: data.result?.stats ?? null,
          columns: data.result?.columns ?? [],
        }
        rawOutput.value = JSON.stringify(payload, null, 2)
        streamMeta.value = { lines: 0, records: 0, errors: 0, previewOnly: false, previewLimit: 200 }
        actionError.value = ''
        lastQueryErrorCode.value = ''
      } else if (queryMode.value === 'columns') {
        const data = await apiPost<{ columns: unknown[]; stats?: QueryStatsData }>(
          '/api/v1/data/query/columns',
          { query },
          { signal },
        )
        if (seq !== requestSeq) return
        rows.value = []
        columnSeries.value = data.columns ?? []
        rawOutput.value = JSON.stringify(data.columns, null, 2)
        streamMeta.value = { lines: 0, records: 0, errors: 0, previewOnly: false, previewLimit: 200 }
        queryStats.value = data.stats ?? null
        engineStatsSource.value = 'query'
        actionError.value = ''
        lastQueryErrorCode.value = ''
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
        rows.value = []
        columnSeries.value = []
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
        queryStats.value = endStats
        engineStatsSource.value = 'query'
        if (streamError) {
          actionError.value = streamError
          lastQueryErrorCode.value = 'stream'
        } else {
          actionError.value = ''
          lastQueryErrorCode.value = ''
        }
      }
    } catch (e) {
      if (seq !== requestSeq) return
      lastQueryErrorCode.value = resolveCaughtErrorCode(e)
      actionError.value = formatCaughtError(e)
      // 失败/取消：保留上次成功快照，避免结果区闪空
      if (!hasQuerySnapshot()) {
        clearQuerySnapshot()
      }
    } finally {
      if (seq === requestSeq) {
        loading.value = false
        queryStartedAt.value = null
        queryAbort = null
      }
    }
  }

  async function loadEngineStats() {
    engineStatsLoading.value = true
    engineStatsError.value = ''
    try {
      const stats = await fetchEngineQueryStats()
      queryStats.value = stats
      engineStatsSource.value = 'engine'
      engineStatsAt.value = Date.now()
    } catch (e) {
      engineStatsError.value = formatCaughtError(e)
    } finally {
      engineStatsLoading.value = false
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
    seriesOffset,
    seriesHasMore,
    SERIES_CAP,
    loadMeasurementMeta,
    refreshSeriesWithTags,
    loadMoreSeries,
    refreshSeriesWithServerQuery,
    applySeriesTags,
    metaSource,
    metaHint,
    queryForm,
    queryMode,
    rows,
    columnSeries,
    queryStats,
    engineStatsSource,
    engineStatsLoading,
    engineStatsError,
    engineStatsAt,
    loadEngineStats,
    rawOutput,
    streamMeta,
    actionError,
    lastQueryErrorCode,
    loading,
    queryStartedAt,
    loadDatabases,
    loadDbChildren,
    hasQuerySnapshot,
    executeQuery,
    cancelQuery,
    resultTextForCopy,
    buildQuery,
  }
}
