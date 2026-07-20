<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { hashTargetId, scheduleScrollToHash } from '@/utils/hashScroll'
import { parseWritePrefill, writeFormToPrefill } from '@/utils/routePrefill'
import { copyText } from '@/utils/clipboard'
import { apiPost } from '@/api/client'
import {
  listDatabasesDetailed,
  listFields,
  listMeasurements,
  listRetentionPoliciesDetailed,
  type MetaLoadSource,
} from '@/api/meta'
import { fieldNames } from '@/utils/seriesMeta'
import { checkDatabasePermission } from '@/api/authz'
import { useAuth } from '@/composables/useAuth'
import { nowUnixMsString } from '@/utils/time'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { formatMessage } from '@/utils/formatMessage'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { Send, Plus, Trash2, Download } from 'lucide-vue-next'
import EmptyState from '@/components/EmptyState.vue'
import { isDirty, snapshotForm } from '@/utils/formDirty'
import { registerDirtyChecker } from '@/utils/routeDirty'
import {
  fieldTypes, buildFormPoints, parseLineProtocolDetailed, parsePrometheusText, type FormRow,
} from '@/composables/usePointParsers'
import { loadWritePrefs, saveWritePrefs, type WriteModePref } from '@/utils/writePrefs'
import { buildWriteDraftExport, buildWriteResultExport } from '@/utils/writeExport'
import { downloadJSON, stampFilename } from '@/utils/download'
import { makeFormErrorT } from '@/utils/formErrors'

type WriteMode = 'form' | 'line' | 'prometheus' | 'typed'

const route = useRoute()
useHashScroll()

const databases = ref<string[]>([])
const retentionPolicies = ref<string[]>([])
const measurements = ref<string[]>([])
const fieldOptions = ref<string[]>([])
const measurementsLoading = ref(false)
const fieldsLoading = ref(false)
const writeMetaError = ref('')
const selectedDb = ref('')
const retentionPolicy = ref('autogen')
const initialWritePrefs = loadWritePrefs(typeof localStorage !== 'undefined' ? localStorage : null)
const syncWrite = ref(initialWritePrefs.syncWrite)
const usePointsTyped = ref(initialWritePrefs.usePointsTyped)
const writeMode = ref<WriteMode>(initialWritePrefs.writeMode as WriteMode)

const WRITE_MODES: WriteMode[] = ['form', 'line', 'prometheus', 'typed']
function applyWriteHash(hash?: string | null) {
  const raw = hash ?? (typeof window !== 'undefined' ? window.location.hash : route.hash)
  const id = hashTargetId(raw)
  if (id.startsWith('write-mode-')) {
    const mode = id.slice('write-mode-'.length) as WriteMode
    if (WRITE_MODES.includes(mode)) writeMode.value = mode
  }
  void nextTick(() => {
    scheduleScrollToHash(raw)
  })
}
watch(
  () => route.hash,
  (h) => applyWriteHash(h),
  { immediate: true },
)

const lineInput = ref('')
const formRows = ref<FormRow[]>([createEmptyRow()])
/** 表单写行数上限；大批量请用 Line Protocol / TypedBatch */
const WRITE_FORM_ROW_MAX = 50
const formRowCapReached = computed(() => formRows.value.length >= WRITE_FORM_ROW_MAX)
const result = ref<{ ok: boolean; message: string } | null>(null)
const loading = ref(false)
const actionError = ref('')
const metaHint = ref('')
const rpMetaHint = ref('')
const metaSource = ref<MetaLoadSource>('admin')
const { success, error: notifyError, warn } = useNotify()
const { t } = useI18n()

function fieldTypeLabel(value: string): string {
  switch (value) {
    case 'float':
      return t.value('typeFloat')
    case 'int':
      return t.value('typeInt')
    case 'string':
      return t.value('typeString')
    case 'bool':
      return t.value('typeBool')
    default:
      return value
  }
}
function trErr(key: MessageKey, vars: Record<string, string | number> = {}) {
  return formatMessage(t.value(key), vars)
}
function writeFormT() {
  return makeFormErrorT({
    writeFormErrBadFieldType: t.value('writeFormErrBadFieldType'),
    writeFormErrBadInt: t.value('writeFormErrBadInt'),
    writeFormErrIntOverflow: t.value('writeFormErrIntOverflow'),
    writeFormErrBadFloat: t.value('writeFormErrBadFloat'),
    writeFormErrNoFields: t.value('writeFormErrNoFields'),
    writeFormErrTsNotInt: t.value('writeFormErrTsNotInt'),
    writeFormErrTsOverflow: t.value('writeFormErrTsOverflow'),
    writeLineErrBadFormat: t.value('writeLineErrBadFormat'),
    writeLineErrBadTag: t.value('writeLineErrBadTag'),
    writeLineErrFieldNoEq: t.value('writeLineErrFieldNoEq'),
    writeLineErrFieldNameEmpty: t.value('writeLineErrFieldNameEmpty'),
    writeLineErrFieldValue: t.value('writeLineErrFieldValue'),
    writeLineErrNoFields: t.value('writeLineErrNoFields'),
    writeLineErrTsOverflow: t.value('writeLineErrTsOverflow'),
    writeLineErrSummaryMore: t.value('writeLineErrSummaryMore'),
  })
}
const { currentUser, isAdmin } = useAuth()
const authzHint = ref('')

// TypedBatch builder（多 tag 列 + 多 field 列）
const typedMeasurement = ref('cpu')
const typedTimestamps = ref('')
type TypedTagCol = { name: string; values: string }
type TypedFieldCol = { name: string; type: 'float' | 'int' | 'string' | 'bool'; values: string }
const typedTagCols = ref<TypedTagCol[]>([{ name: 'host', values: 'server01\nserver02' }])
const typedFieldCols = ref<TypedFieldCol[]>([{ name: 'usage', type: 'float', values: '0.7\n0.8' }])


function writeSnapshot() {
  return {
    selectedDb: selectedDb.value,
    retentionPolicy: retentionPolicy.value,
    writeMode: writeMode.value,
    lineInput: lineInput.value,
    formRows: formRows.value,
    typedMeasurement: typedMeasurement.value,
    typedTimestamps: typedTimestamps.value,
    typedTagCols: typedTagCols.value,
    typedFieldCols: typedFieldCols.value,
  }
}
const writeBaseline = ref(snapshotForm(writeSnapshot()))
const formDirty = computed(() => isDirty(writeBaseline.value, writeSnapshot()))
watch(
  [writeMode, usePointsTyped, syncWrite],
  () => {
    saveWritePrefs(typeof localStorage !== 'undefined' ? localStorage : null, {
      writeMode: writeMode.value as WriteModePref,
      usePointsTyped: usePointsTyped.value,
      syncWrite: syncWrite.value,
    })
  },
  { deep: false },
)
function markWriteClean() {
  writeBaseline.value = snapshotForm(writeSnapshot())
}
function onBeforeUnload(e: BeforeUnloadEvent) {
  if (!formDirty.value) return
  e.preventDefault()
  e.returnValue = ''
}

function createEmptyRow(): FormRow {
  return {
    measurement: 'cpu',
    tags: [{ key: 'host', value: 'server01' }],
    fields: [{ key: 'usage', value: '0.75', type: 'float' }],
    timestamp: nowUnixMsString(),
  }
}

let unregisterDirty: (() => void) | null = null

onMounted(async () => {
  unregisterDirty = registerDirtyChecker('write', () => formDirty.value)
  window.addEventListener('beforeunload', onBeforeUnload)
  const result = await listDatabasesDetailed()
  databases.value = result.names
  metaSource.value = result.source
  metaHint.value = result.error || (result.source === 'manual' ? t.value('writeManualDbHint') : '')
  if (databases.value.length && !selectedDb.value) {
    selectedDb.value = databases.value[0]
  }
  markWriteClean()
})
onBeforeUnmount(() => {
  unregisterDirty?.()
  unregisterDirty = null
  window.removeEventListener('beforeunload', onBeforeUnload)
})

watch(selectedDb, async (db) => {
  retentionPolicies.value = []
  retentionPolicy.value = 'autogen'
  rpMetaHint.value = ''
  measurements.value = []
  fieldOptions.value = []
  writeMetaError.value = ''
  if (!db) return
  measurementsLoading.value = true
  try {
    try {
      const rpResult = await listRetentionPoliciesDetailed(db)
      retentionPolicies.value = rpResult.policies.map((p) => p.name)
      if (retentionPolicies.value.length) {
        retentionPolicy.value = retentionPolicies.value[0]
        rpMetaHint.value = ''
      } else if (rpResult.source === 'manual') {
        rpMetaHint.value = t.value('writeRpManualHint')
      } else {
        rpMetaHint.value = t.value('writeRpEmptyHint')
      }
    } catch {
      rpMetaHint.value = t.value('writeRpManualHint')
    }
    try {
      measurements.value = await listMeasurements(db)
    } catch (e) {
      measurements.value = []
      writeMetaError.value = formatCaughtError(e)
    }
  } finally {
    measurementsLoading.value = false
  }
  // 自动填充 RP 不应算用户脏编辑
  markWriteClean()
})

async function loadWriteFields(measurement: string) {
  fieldOptions.value = []
  const db = selectedDb.value.trim()
  const m = measurement.trim()
  if (!db || !m) return
  fieldsLoading.value = true
  try {
    const fields = await listFields(db, m)
    fieldOptions.value = fieldNames(fields)
  } catch (e) {
    fieldOptions.value = []
    writeMetaError.value = formatCaughtError(e)
  } finally {
    fieldsLoading.value = false
  }
}

function onFormMeasurementBlur(row: FormRow) {
  void loadWriteFields(row.measurement)
}

function onTypedMeasurementBlur() {
  void loadWriteFields(typedMeasurement.value)
}

// 预填或切换库/表后自动拉 field 建议（不依赖 blur）
watch(
  () => [selectedDb.value, typedMeasurement.value] as const,
  ([db, m]) => {
    if (db.trim() && m.trim()) void loadWriteFields(m)
  },
)

function applyWritePrefillFromRoute() {
  const pre = parseWritePrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.database && selectedDb.value !== pre.database) {
    selectedDb.value = pre.database
    changed = true
  }
  if (pre.measurement) {
    if (typedMeasurement.value !== pre.measurement) {
      typedMeasurement.value = pre.measurement
      changed = true
    }
    if (formRows.value[0] && formRows.value[0].measurement !== pre.measurement) {
      formRows.value[0].measurement = pre.measurement
      changed = true
    }
  }
  if (changed) {
    success(t.value('writePrefillApplied'))
  }
}

async function copyWriteShareLink() {
  const measurement =
    writeMode.value === 'form'
      ? formRows.value[0]?.measurement
      : typedMeasurement.value
  const path = writeFormToPrefill(
    { database: selectedDb.value, measurement },
    { hash: writeMode.value === 'typed' ? '#write-mode-typed' : '#write-target' },
  )
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('writeShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

watch(
  () => route.fullPath,
  () => {
    applyWritePrefillFromRoute()
  },
  { immediate: true },
)

function applyFieldSuggestion(name: string, row: FormRow) {
  const token = name.trim()
  if (!token) return
  if (row.fields.some((f) => f.key.trim() === token)) return
  // 优先填充第一个空 key
  const empty = row.fields.find((f) => !f.key.trim())
  if (empty) {
    empty.key = token
    return
  }
  row.fields.push({ key: token, value: '', type: 'float' })
}

function applyTypedFieldSuggestion(name: string) {
  const token = name.trim()
  if (!token) return
  if (typedFieldCols.value.some((c) => c.name.trim() === token)) return
  const empty = typedFieldCols.value.find((c) => !c.name.trim())
  if (empty) {
    empty.name = token
    return
  }
  typedFieldCols.value.push({ name: token, type: 'float', values: '' })
}

function applyMeasurementSuggestion(name: string) {
  const token = name.trim()
  if (!token) return
  typedMeasurement.value = token
  if (formRows.value[0]) formRows.value[0].measurement = token
  void loadWriteFields(token)
}

function applyFieldChip(name: string) {
  if (writeMode.value === 'typed') {
    applyTypedFieldSuggestion(name)
    return
  }
  if (formRows.value[0]) applyFieldSuggestion(name, formRows.value[0])
}

function buildTypedBatch(): Record<string, unknown> {
  if (!typedMeasurement.value.trim()) throw new Error(trErr('writeErrMeasurementRequired'))
  const fieldColsBuilt: Record<string, unknown>[] = []
  let n = 0
  for (const col of typedFieldCols.value) {
    const name = col.name.trim()
    if (!name) throw new Error(trErr('writeErrFieldNameRequired'))
    const vals = col.values.split('\n').map((s) => s.trim()).filter(Boolean)
    if (!vals.length) throw new Error(trErr('writeErrFieldValuesEmpty', { name }))
    if (!n) n = vals.length
    else if (vals.length !== n) throw new Error(trErr('writeErrFieldRowMismatch', { name, got: vals.length, want: n }))
    const fieldCol: Record<string, unknown> = { name }
    if (col.type === 'int') {
      fieldCol.type = 2
      fieldCol.int64_values = vals.map((v) => {
        if (!/^-?\d+$/.test(v)) throw new Error(trErr('writeErrBadInt', { value: v }))
        const num = Number(v)
        if (!Number.isSafeInteger(num)) throw new Error(trErr('writeErrIntOverflow', { value: v }))
        return num
      })
    } else if (col.type === 'float') {
      fieldCol.type = 1
      fieldCol.float64_values = vals.map((v) => {
        const num = Number(v)
        if (!Number.isFinite(num)) throw new Error(trErr('writeErrBadFloat', { value: v }))
        return num
      })
    } else if (col.type === 'bool') {
      fieldCol.type = 4
      fieldCol.bool_values = vals.map((v) => {
        const s = v.toLowerCase()
        if (s === 'true' || s === '1') return true
        if (s === 'false' || s === '0') return false
        throw new Error(trErr('writeErrBadBool', { value: v }))
      })
    } else {
      fieldCol.type = 3
      fieldCol.string_values = vals
    }
    fieldColsBuilt.push(fieldCol)
  }
  if (!n) throw new Error(trErr('writeErrNeedFieldCol'))
  let ts = typedTimestamps.value.split('\n').map((s) => s.trim()).filter(Boolean).map(Number)
  if (!ts.length) {
    const now = Date.now()
    ts = Array.from({ length: n }, (_, i) => now + i)
  }
  if (ts.length !== n) throw new Error(trErr('writeErrTsRowMismatch'))
  if (ts.some((x) => !Number.isSafeInteger(x))) throw new Error(trErr('writeErrTsNotInt'))
  const tagCols: { name: string; values: string[] }[] = []
  for (const col of typedTagCols.value) {
    const name = col.name.trim()
    if (!name) continue
    const values = col.values.split('\n').map((s) => s.trim()).filter(Boolean)
    if (!values.length) continue
    if (values.length !== n) throw new Error(trErr('writeErrTagRowMismatch', { name, want: n }))
    tagCols.push({ name, values })
  }
  const batch: Record<string, unknown> = {
    database: selectedDb.value,
    retention_policy: retentionPolicy.value,
    measurement: typedMeasurement.value.trim(),
    precision: 'ms',
    timestamps: ts,
    fields: fieldColsBuilt,
  }
  if (tagCols.length) batch.tags = tagCols
  return batch
}

async function checkWriteAuthz() {
  authzHint.value = ''
  if (!selectedDb.value.trim()) {
    authzHint.value = t.value('writeNeedDatabase')
    notifyError(authzHint.value)
    return
  }
  try {
    const allowed = await checkDatabasePermission({
      database: selectedDb.value.trim(),
      permission: 'write',
      user_name: isAdmin.value ? undefined : currentUser.value || undefined,
    })
    authzHint.value = allowed
      ? formatMessage(t.value('writeAuthzPassShort'), { db: selectedDb.value })
      : formatMessage(t.value('writeAuthzDenyShort'), { db: selectedDb.value })
    if (allowed) success(authzHint.value)
    else notifyError(authzHint.value)
  } catch (e) {
    authzHint.value = formatCaughtError(e)
    notifyError(authzHint.value)
  }
}

async function submit() {
  loading.value = true
  actionError.value = ''
  result.value = null
  try {
    if (writeMode.value === 'typed') {
      const batch = buildTypedBatch()
      await apiPost('/api/v1/data/write/typed', { batch, options: { sync: syncWrite.value } })
      result.value = { ok: true, message: formatMessage(t.value('writeTypedSuccess'), { count: (batch.timestamps as number[]).length }) }
      success(result.value.message)
      markWriteClean()
      return
    }
    let points: Record<string, unknown>[] = []
    if (writeMode.value === 'form') {
      points = buildFormPoints(formRows.value, writeFormT())
    } else if (writeMode.value === 'line') {
      const parsed = parseLineProtocolDetailed(lineInput.value, writeFormT())
      if (parsed.errors.length) {
        const summary = parsed.errors.slice(0, 5).join('; ') + (parsed.errors.length > 5 ? formatMessage(t.value('writeLineMoreErrors'), { count: parsed.errors.length }) : '')
        if (!parsed.points.length) throw new Error(summary)
        notifyError(formatMessage(t.value('writeLinePartialInvalid'), { summary }))
      }
      points = parsed.points
    } else {
      points = parsePrometheusText(lineInput.value)
    }
    if (!points.length) throw new Error(trErr('writeErrNoPoints'))
    for (const p of points) {
      p.database = selectedDb.value
      p.retention_policy = retentionPolicy.value
    }
    const writePath = usePointsTyped.value ? '/api/v1/data/write/points-typed' : '/api/v1/data/write'
    await apiPost(writePath, { points, options: { sync: syncWrite.value } })
    result.value = { ok: true, message: formatMessage(t.value('writeSuccessPoints'), { count: points.length, path: writePath }) }
    success(result.value.message)
    markWriteClean()
  } catch (e) {
    actionError.value = formatCaughtError(e)
    notifyError(actionError.value)
    result.value = { ok: false, message: actionError.value }
  } finally {
    loading.value = false
  }
}

function addTypedTagCol() { typedTagCols.value.push({ name: '', values: '' }) }
function removeTypedTagCol(i: number) { typedTagCols.value.splice(i, 1) }
function addTypedFieldCol() { typedFieldCols.value.push({ name: '', type: 'float', values: '' }) }
function removeTypedFieldCol(i: number) { typedFieldCols.value.splice(i, 1) }
function addRow() {
  if (formRows.value.length >= WRITE_FORM_ROW_MAX) {
    notifyError(formatMessage(t.value('writeFormRowLimit'), { max: WRITE_FORM_ROW_MAX }))
    return
  }
  formRows.value.push(createEmptyRow())
}
function removeRow(i: number) { formRows.value.splice(i, 1) }
const modeLabel = computed(() => ({
  form: t.value('formWrite'),
  line: t.value('lineProtocol'),
  prometheus: t.value('prometheus'),
  typed: t.value('typedBatch'),
}[writeMode.value]))

function exportWriteResult() {
  if (!result.value) {
    warn(t.value('writeResultExportEmpty'))
    return
  }
  downloadJSON(
    stampFilename('mts-write-result', 'json'),
    buildWriteResultExport({
      ok: result.value.ok,
      message: result.value.message,
      mode: writeMode.value,
      database: selectedDb.value,
      retention_policy: retentionPolicy.value,
      sync: syncWrite.value,
      use_points_typed: usePointsTyped.value,
    }),
  )
  success(t.value('writeResultExported'))
}

function exportWriteDraft() {
  downloadJSON(
    stampFilename('mts-write-draft', 'json'),
    buildWriteDraftExport({
      mode: writeMode.value,
      database: selectedDb.value,
      retention_policy: retentionPolicy.value,
      line_input: lineInput.value,
      form_rows: formRows.value,
      typed: {
        measurement: typedMeasurement.value,
        timestamps: typedTimestamps.value,
        tags: typedTagCols.value,
        fields: typedFieldCols.value,
      },
    }),
  )
  success(t.value('writeDraftExported'))
}
</script>

<template>
  <div class="space-y-4" data-testid="write-page">
    <div class="space-y-2">
      <div id="write-mode-tabs" class="scroll-mt-20 flex flex-wrap gap-2" data-testid="write-mode-tabs">
        <button
          v-for="m in (['form','line','prometheus','typed'] as const)"
          :key="m"
          type="button"
          class="scroll-mt-20 rounded-lg border px-3 py-1.5 text-xs"
          :id="`write-mode-${m}`"
          :data-testid="`write-mode-${m}`"
          :class="writeMode===m ? 'border-slate-800 bg-slate-800 text-white dark:bg-slate-100 dark:text-slate-900' : 'border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900'"
          @click="writeMode=m"
        >
          {{ ({form:t('formWrite'), line:t('lineProtocol'), prometheus:t('prometheus'), typed:t('typedBatch')})[m] }}
          <span v-if="m==='typed'" class="ml-1 rounded bg-emerald-500/20 px-1 text-[10px] text-emerald-700 dark:text-emerald-300">{{ t('writePreferTypedBadge') }}</span>
        </button>
      </div>
      <p class="text-[11px] mts-muted" data-testid="write-prefs-hint">{{ t('writeModeRemembered') }}</p>
    </div>

    <div id="write-target" class="scroll-mt-20 grid gap-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900 md:grid-cols-4">
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('database') }}
        <input v-model="selectedDb" list="write-db-list" class="mt-1 w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" :placeholder="t('writePhDatabase')" />
        <datalist id="write-db-list"><option v-for="db in databases" :key="db" :value="db" /></datalist>
        <datalist id="write-meas-list"><option v-for="m in measurements" :key="m" :value="m" /></datalist>
        <datalist id="write-field-list"><option v-for="f in fieldOptions" :key="f" :value="f" /></datalist>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('retentionPolicy') }}
        <input v-model="retentionPolicy" list="write-rp-list" data-testid="write-retention-policy" class="mt-1 w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" :placeholder="t('writePhAutogen')" />
        <datalist id="write-rp-list"><option v-for="rp in (retentionPolicies.length?retentionPolicies:['autogen'])" :key="rp" :value="rp" /></datalist>
        <p v-if="rpMetaHint" class="mt-1 text-[11px] text-amber-700 dark:text-amber-200" data-testid="write-rp-meta-hint">{{ rpMetaHint }}</p>
      </label>
      <label class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300 md:mt-6">
        <input v-model="syncWrite" type="checkbox" /> {{ t('writeSync') }}
      </label>
      <label v-if="writeMode!=='typed'" class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300 md:mt-6">
        <input v-model="usePointsTyped" type="checkbox" /> {{ t('writePointsTypedPath') }}
      </label>
    </div>
    <p v-if="metaHint" class="rounded-xl border border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/40 p-3 text-sm text-amber-800 dark:text-amber-200 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200">{{ metaHint }}</p>
    <div
      v-if="selectedDb"
      class="rounded-xl border border-slate-200 bg-white p-3 text-xs dark:border-slate-700 dark:bg-slate-900"
      data-testid="write-meta-panel"
    >
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <span class="font-medium text-slate-700 dark:text-slate-200">{{ t('writeMetaTitle') }}</span>
        <span class="mts-muted" data-testid="write-meta-count">
          {{ formatMessage(t('writeMetaCount'), { meas: measurements.length, fields: fieldOptions.length }) }}
        </span>
      </div>
      <p v-if="measurementsLoading || fieldsLoading" class="mts-muted">{{ t('loading') }}</p>
      <p v-else-if="writeMetaError" class="text-rose-600" data-testid="write-meta-error">{{ writeMetaError }}</p>
      <div v-else class="space-y-2">
        <div>
          <p class="mb-1 mts-muted">{{ t('writeMetaMeasurements') }}</p>
          <div v-if="measurements.length" class="flex flex-wrap gap-1" data-testid="write-meas-chips">
            <button
              v-for="m in measurements.slice(0, 24)"
              :key="m"
              type="button"
              class="rounded-full border border-slate-200 px-2 py-0.5 font-mono text-[11px] hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800"
              @click="applyMeasurementSuggestion(m)"
            >{{ m }}</button>
          </div>
          <p v-else class="mts-muted" data-testid="write-meas-empty">{{ t('writeMetaMeasEmpty') }}</p>
        </div>
        <div>
          <p class="mb-1 mts-muted">{{ t('writeMetaFields') }}</p>
          <div v-if="fieldOptions.length" class="flex flex-wrap gap-1" data-testid="write-field-chips">
            <button
              v-for="f in fieldOptions.slice(0, 32)"
              :key="f"
              type="button"
              class="rounded-full border border-slate-200 px-2 py-0.5 font-mono text-[11px] hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800"
              @click="applyFieldChip(f)"
            >{{ f }}</button>
          </div>
          <p v-else class="mts-muted" data-testid="write-field-empty">{{ t('writeMetaFieldEmpty') }}</p>
        </div>
      </div>
      <p class="mt-2 text-[11px] mts-muted">{{ t('writeMetaHint') }}</p>
    </div>

    <div id="write-body" v-if="writeMode==='form'" class="scroll-mt-20 space-y-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900">
      <div v-for="(row, idx) in formRows" :key="idx" class="rounded border border-slate-100 p-3 dark:border-slate-800">
        <div class="mb-2 flex justify-between text-xs font-medium">{{ formatMessage(t('writeRowN'), { n: idx+1 }) }}
          <button class="text-red-500" @click="removeRow(idx)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <div class="grid gap-2 md:grid-cols-3">
          <input
            v-model="row.measurement"
            list="write-meas-list"
            data-testid="write-form-measurement"
            :placeholder="t('writePhMeasurement')"
            class="rounded border px-2 py-1 text-xs dark:border-slate-600 dark:bg-slate-800"
            @blur="onFormMeasurementBlur(row)"
          />
          <input v-model="row.timestamp" :placeholder="t('writePhTimestampMs')" class="rounded border px-2 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" />
          <div class="text-[11px] text-slate-400 dark:text-slate-500">{{ t('writeFormTagsFieldsHint') }}</div>
        </div>
        <div class="mt-2 grid gap-2 md:grid-cols-2">
          <div>
            <p class="mb-1 text-[11px] text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('tags') }}</p>
            <div v-for="(tg, ti) in row.tags" :key="ti" class="mb-1 flex gap-1">
              <input v-model="tg.key" class="w-1/2 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" :placeholder="t('writePhKey')" />
              <input v-model="tg.value" class="w-1/2 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" :placeholder="t('writePhValue')" />
            </div>
            <button class="text-[11px] text-slate-500 dark:text-slate-400 dark:text-slate-500" @click="row.tags.push({key:'',value:''})">{{ t('writeAddTag') }}</button>
          </div>
          <div>
            <p class="mb-1 text-[11px] text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('fields') }}</p>
            <div v-for="(fd, fi) in row.fields" :key="fi" class="mb-1 flex gap-1">
              <input
                v-model="fd.key"
                list="write-field-list"
                class="w-1/3 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800"
                :placeholder="t('writePhKey')"
              />
              <input v-model="fd.value" class="w-1/3 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" :placeholder="t('writePhValue')" />
              <select v-model="fd.type" class="w-1/3 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800">
                <option v-for="ft in fieldTypes" :key="ft.value" :value="ft.value">{{ fieldTypeLabel(ft.value) }}</option>
              </select>
            </div>
            <button class="text-[11px] text-slate-500 dark:text-slate-400 dark:text-slate-500" @click="row.fields.push({key:'',value:'',type:'float'})">{{ t('writeAddField') }}</button>
          </div>
        </div>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="inline-flex items-center gap-1 text-xs text-slate-600 disabled:opacity-50 dark:text-slate-300"
          data-testid="write-add-row"
          :disabled="formRowCapReached"
          @click="addRow"
        ><Plus class="h-3 w-3" /> {{ t('writeAddRow') }}</button>
        <span class="text-[11px] mts-muted" data-testid="write-form-row-count">{{ formRows.length }}/{{ WRITE_FORM_ROW_MAX }}</span>
        <span v-if="formRowCapReached" class="text-[11px] text-amber-700 dark:text-amber-200" data-testid="write-form-row-limit-hint">{{ formatMessage(t('writeFormRowLimitHint'), { max: WRITE_FORM_ROW_MAX }) }}</span>
      </div>
    </div>

    <div id="write-body" v-else-if="writeMode==='typed'" class="scroll-mt-20 space-y-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900">
      <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('writeTypedHint') }}</p>
      <div class="grid gap-3 md:grid-cols-2">
        <label class="text-xs">{{ t('measurement') }}
          <input
            v-model="typedMeasurement"
            list="write-meas-list"
            data-testid="write-typed-measurement"
            class="mts-input mt-1"
            @blur="onTypedMeasurementBlur"
          />
        </label>
        <label class="text-xs">{{ t('writeTimestampsMs') }}
          <textarea v-model="typedTimestamps" rows="3" class="mts-input mt-1 font-mono text-xs" />
        </label>
      </div>
      <div>
        <div class="mb-2 flex items-center justify-between text-xs font-medium">{{ t('writeTagCols') }}
          <button type="button" class="text-slate-500" @click="addTypedTagCol">{{ t('writeAddTagCol') }}</button>
        </div>
        <div v-for="(col, i) in typedTagCols" :key="'t'+i" class="mb-2 grid gap-2 rounded border border-slate-100 p-2 dark:border-slate-800 md:grid-cols-3">
          <input v-model="col.name" class="mts-input text-xs" :placeholder="t('writePhTagName')" />
          <textarea v-model="col.values" rows="3" class="mts-input font-mono text-xs md:col-span-2" :placeholder="t('writePerLineValue')" />
          <button type="button" class="text-xs text-red-500 md:col-span-3" @click="removeTypedTagCol(i)">{{ t('writeRemoveTagCol') }}</button>
        </div>
      </div>
      <div>
        <div class="mb-2 flex items-center justify-between text-xs font-medium">{{ t('writeFieldCols') }}
          <button type="button" class="text-slate-500" @click="addTypedFieldCol">{{ t('writeAddFieldCol') }}</button>
        </div>
        <div v-for="(col, i) in typedFieldCols" :key="'f'+i" class="mb-2 grid gap-2 rounded border border-slate-100 p-2 dark:border-slate-800 md:grid-cols-4">
          <input v-model="col.name" list="write-field-list" class="mts-input text-xs" :placeholder="t('writePhFieldName')" />
          <select v-model="col.type" class="mts-input text-xs">
            <option v-for="ft in fieldTypes" :key="ft.value" :value="ft.value">{{ fieldTypeLabel(ft.value) }}</option>
          </select>
          <textarea v-model="col.values" rows="3" class="mts-input font-mono text-xs md:col-span-2" :placeholder="t('writePerLineValue')" />
          <button type="button" class="text-xs text-red-500 md:col-span-4" :disabled="typedFieldCols.length<=1" @click="removeTypedFieldCol(i)">{{ t('writeRemoveFieldCol') }}</button>
        </div>
      </div>
    </div>

    <div id="write-body" v-else class="scroll-mt-20 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900">
      <textarea v-model="lineInput" rows="10" class="w-full rounded border border-slate-300 dark:border-slate-600 px-3 py-2 font-mono text-xs dark:border-slate-600 dark:bg-slate-800" :placeholder="modeLabel" />
    </div>

    <div id="write-actions" class="scroll-mt-20 flex flex-wrap items-center gap-2">
      <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-slate-100 dark:bg-slate-800 dark:text-slate-900" data-testid="write-submit" :disabled="loading" @click="submit">
        <Send class="h-4 w-4" /> {{ loading ? t('loading') : t('writeSubmit') }}
      </button>
      <button type="button" class="mts-btn" data-testid="write-export-result" :disabled="!result" @click="exportWriteResult">
        <Download class="h-3.5 w-3.5" /> {{ t('writeExportResult') }}
      </button>
      <button type="button" class="mts-btn" data-testid="write-export-draft" @click="exportWriteDraft">
        <Download class="h-3.5 w-3.5" /> {{ t('writeExportDraft') }}
      </button>
      <button type="button" class="mts-btn" data-testid="write-share-link" @click="copyWriteShareLink">
        {{ t('writeShareLink') }}
      </button>
      <span
        v-if="formDirty"
        class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
      >{{ t('writeDirtyBadge') }}</span>
    </div>
    <p v-if="actionError" class="mts-alert-error">{{ actionError }}</p>
    <p v-if="result?.ok" class="mts-alert-ok" data-testid="write-result-ok">{{ result.message }}</p>
    <div v-else-if="!loading && !actionError && !result" class="mts-card">
      <EmptyState
        compact
        :title="t('writeEmptyTitle')"
        :description="t('writeEmptyDesc')"
      />
    </div>
  </div>
</template>
