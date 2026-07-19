<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { apiPost } from '@/api/client'
import { listDatabasesDetailed, listRetentionPolicies } from '@/api/meta'
import { checkDatabasePermission } from '@/api/authz'
import { useAuth } from '@/composables/useAuth'
import { nowUnixMsString } from '@/utils/time'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { formatMessage } from '@/utils/formatMessage'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { Send, Plus, Trash2 } from 'lucide-vue-next'
import EmptyState from '@/components/EmptyState.vue'
import { isDirty, snapshotForm } from '@/utils/formDirty'
import { registerDirtyChecker } from '@/utils/routeDirty'
import {
  fieldTypes, buildFormPoints, parseLineProtocolDetailed, parsePrometheusText, type FormRow,
} from '@/composables/usePointParsers'

type WriteMode = 'form' | 'line' | 'prometheus' | 'typed'

const databases = ref<string[]>([])
const retentionPolicies = ref<string[]>([])
const selectedDb = ref('')
const retentionPolicy = ref('autogen')
const syncWrite = ref(false)
const usePointsTyped = ref(true)
const writeMode = ref<WriteMode>('form')
const lineInput = ref('')
const formRows = ref<FormRow[]>([createEmptyRow()])
const result = ref<{ ok: boolean; message: string } | null>(null)
const loading = ref(false)
const actionError = ref('')
const metaHint = ref('')
const { success, error: notifyError } = useNotify()
const { t } = useI18n()
function trErr(key: MessageKey, vars: Record<string, string | number> = {}) {
  return formatMessage(t.value(key), vars)
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
  if (!db) return
  try {
    const rps = await listRetentionPolicies(db)
    retentionPolicies.value = rps.map((p) => p.name)
    if (retentionPolicies.value.length) retentionPolicy.value = retentionPolicies.value[0]
  } catch { /* ignore */ }
  // 自动填充 RP 不应算用户脏编辑
  markWriteClean()
})

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
      points = buildFormPoints(formRows.value)
    } else if (writeMode.value === 'line') {
      const parsed = parseLineProtocolDetailed(lineInput.value)
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
function addRow() { formRows.value.push(createEmptyRow()) }
function removeRow(i: number) { formRows.value.splice(i, 1) }
const modeLabel = computed(() => ({
  form: t.value('formWrite'),
  line: t.value('lineProtocol'),
  prometheus: t.value('prometheus'),
  typed: t.value('typedBatch'),
}[writeMode.value]))
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap gap-2">
      <button v-for="m in (['form','line','prometheus','typed'] as const)" :key="m"
        class="rounded-lg border px-3 py-1.5 text-xs"
        :class="writeMode===m ? 'border-slate-800 bg-slate-800 text-white dark:bg-slate-100 dark:text-slate-900' : 'border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900'"
        @click="writeMode=m">{{ ({form:t('formWrite'), line:t('lineProtocol'), prometheus:t('prometheus'), typed:t('typedBatch')})[m] }}</button>
    </div>

    <div class="grid gap-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900 md:grid-cols-4">
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">Database
        <input v-model="selectedDb" list="write-db-list" class="mt-1 w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" placeholder="database" />
        <datalist id="write-db-list"><option v-for="db in databases" :key="db" :value="db" /></datalist>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">RP
        <input v-model="retentionPolicy" list="write-rp-list" class="mt-1 w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" placeholder="autogen" />
        <datalist id="write-rp-list"><option v-for="rp in (retentionPolicies.length?retentionPolicies:['autogen'])" :key="rp" :value="rp" /></datalist>
      </label>
      <label class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300 md:mt-6">
        <input v-model="syncWrite" type="checkbox" /> Sync
      </label>
      <label v-if="writeMode!=='typed'" class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300 md:mt-6">
        <input v-model="usePointsTyped" type="checkbox" /> {{ t('writePointsTypedPath') }}
      </label>
    </div>
    <p v-if="metaHint" class="rounded-xl border border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/40 p-3 text-sm text-amber-800 dark:text-amber-200 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200">{{ metaHint }}</p>

    <div v-if="writeMode==='form'" class="space-y-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900">
      <div v-for="(row, idx) in formRows" :key="idx" class="rounded border border-slate-100 p-3 dark:border-slate-800">
        <div class="mb-2 flex justify-between text-xs font-medium">Row {{ idx+1 }}
          <button class="text-red-500" @click="removeRow(idx)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <div class="grid gap-2 md:grid-cols-3">
          <input v-model="row.measurement" placeholder="measurement" class="rounded border px-2 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" />
          <input v-model="row.timestamp" placeholder="timestamp ms" class="rounded border px-2 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" />
          <div class="text-[11px] text-slate-400 dark:text-slate-500">{{ t('writeFormTagsFieldsHint') }}</div>
        </div>
        <div class="mt-2 grid gap-2 md:grid-cols-2">
          <div>
            <p class="mb-1 text-[11px] text-slate-500 dark:text-slate-400 dark:text-slate-500">Tags</p>
            <div v-for="(tg, ti) in row.tags" :key="ti" class="mb-1 flex gap-1">
              <input v-model="tg.key" class="w-1/2 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" placeholder="key" />
              <input v-model="tg.value" class="w-1/2 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" placeholder="value" />
            </div>
            <button class="text-[11px] text-slate-500 dark:text-slate-400 dark:text-slate-500" @click="row.tags.push({key:'',value:''})">+ tag</button>
          </div>
          <div>
            <p class="mb-1 text-[11px] text-slate-500 dark:text-slate-400 dark:text-slate-500">Fields</p>
            <div v-for="(fd, fi) in row.fields" :key="fi" class="mb-1 flex gap-1">
              <input v-model="fd.key" class="w-1/3 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" placeholder="key" />
              <input v-model="fd.value" class="w-1/3 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" placeholder="value" />
              <select v-model="fd.type" class="w-1/3 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800">
                <option v-for="ft in fieldTypes" :key="ft.value" :value="ft.value">{{ ft.label }}</option>
              </select>
            </div>
            <button class="text-[11px] text-slate-500 dark:text-slate-400 dark:text-slate-500" @click="row.fields.push({key:'',value:'',type:'float'})">+ field</button>
          </div>
        </div>
      </div>
      <button class="inline-flex items-center gap-1 text-xs text-slate-600 dark:text-slate-300" @click="addRow"><Plus class="h-3 w-3" /> {{ t('writeAddRow') }}</button>
    </div>

    <div v-else-if="writeMode==='typed'" class="space-y-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900">
      <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('writeTypedHint') }}</p>
      <div class="grid gap-3 md:grid-cols-2">
        <label class="text-xs">Measurement<input v-model="typedMeasurement" class="mts-input mt-1" /></label>
        <label class="text-xs">{{ t('writeTimestampsMs') }}
          <textarea v-model="typedTimestamps" rows="3" class="mts-input mt-1 font-mono text-xs" />
        </label>
      </div>
      <div>
        <div class="mb-2 flex items-center justify-between text-xs font-medium">{{ t('writeTagCols') }}
          <button type="button" class="text-slate-500" @click="addTypedTagCol">{{ t('writeAddTagCol') }}</button>
        </div>
        <div v-for="(col, i) in typedTagCols" :key="'t'+i" class="mb-2 grid gap-2 rounded border border-slate-100 p-2 dark:border-slate-800 md:grid-cols-3">
          <input v-model="col.name" class="mts-input text-xs" placeholder="tag name" />
          <textarea v-model="col.values" rows="3" class="mts-input font-mono text-xs md:col-span-2" :placeholder="t('writePerLineValue')" />
          <button type="button" class="text-xs text-red-500 md:col-span-3" @click="removeTypedTagCol(i)">{{ t('writeRemoveTagCol') }}</button>
        </div>
      </div>
      <div>
        <div class="mb-2 flex items-center justify-between text-xs font-medium">{{ t('writeFieldCols') }}
          <button type="button" class="text-slate-500" @click="addTypedFieldCol">{{ t('writeAddFieldCol') }}</button>
        </div>
        <div v-for="(col, i) in typedFieldCols" :key="'f'+i" class="mb-2 grid gap-2 rounded border border-slate-100 p-2 dark:border-slate-800 md:grid-cols-4">
          <input v-model="col.name" class="mts-input text-xs" placeholder="field name" />
          <select v-model="col.type" class="mts-input text-xs">
            <option value="float">float</option>
            <option value="int">int</option>
            <option value="string">string</option>
            <option value="bool">bool</option>
          </select>
          <textarea v-model="col.values" rows="3" class="mts-input font-mono text-xs md:col-span-2" :placeholder="t('writePerLineValue')" />
          <button type="button" class="text-xs text-red-500 md:col-span-4" :disabled="typedFieldCols.length<=1" @click="removeTypedFieldCol(i)">{{ t('writeRemoveFieldCol') }}</button>
        </div>
      </div>
    </div>

    <div v-else class="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900">
      <textarea v-model="lineInput" rows="10" class="w-full rounded border border-slate-300 dark:border-slate-600 px-3 py-2 font-mono text-xs dark:border-slate-600 dark:bg-slate-800" :placeholder="modeLabel" />
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-slate-100 dark:bg-slate-800 dark:text-slate-900" :disabled="loading" @click="submit">
        <Send class="h-4 w-4" /> {{ loading ? t('loading') : t('writeSubmit') }}
      </button>
      <span
        v-if="formDirty"
        class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
      >{{ t('writeDirtyBadge') }}</span>
    </div>
    <p v-if="actionError" class="mts-alert-error">{{ actionError }}</p>
    <p v-if="result?.ok" class="mts-alert-ok">{{ result.message }}</p>
    <div v-else-if="!loading && !actionError && !result" class="mts-card">
      <EmptyState
        compact
        :title="t('writeEmptyTitle')"
        :description="t('writeEmptyDesc')"
      />
    </div>
  </div>
</template>
