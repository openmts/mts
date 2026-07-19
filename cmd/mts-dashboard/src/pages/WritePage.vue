<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { apiPost } from '@/api/client'
import { listDatabases, listRetentionPolicies } from '@/api/meta'
import { nowUnixMsString } from '@/utils/time'
import { useNotify } from '@/composables/useNotify'
import { useI18n } from '@/composables/useI18n'
import { Send, Plus, Trash2 } from 'lucide-vue-next'
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
const { success, error: notifyError } = useNotify()
const { t } = useI18n()

// TypedBatch builder
const typedMeasurement = ref('cpu')
const typedTagKey = ref('host')
const typedTagValues = ref('server01\nserver02')
const typedFieldName = ref('usage')
const typedFieldType = ref<'float' | 'int'>('float')
const typedFieldValues = ref('0.7\n0.8')
const typedTimestamps = ref('')

function createEmptyRow(): FormRow {
  return {
    measurement: 'cpu',
    tags: [{ key: 'host', value: 'server01' }],
    fields: [{ key: 'usage', value: '0.75', type: 'float' }],
    timestamp: nowUnixMsString(),
  }
}

onMounted(async () => {
  try {
    databases.value = await listDatabases()
    if (databases.value.length) selectedDb.value = databases.value[0]
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '加载数据库失败'
  }
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
})

function buildTypedBatch(): Record<string, unknown> {
  const tags = typedTagValues.value.split('\n').map((s) => s.trim()).filter(Boolean)
  const vals = typedFieldValues.value.split('\n').map((s) => s.trim()).filter(Boolean)
  if (!typedMeasurement.value.trim()) throw new Error('measurement 不能为空')
  if (!vals.length) throw new Error('字段值不能为空')
  if (tags.length && tags.length !== vals.length) throw new Error('tag 值行数需与字段值行数一致（或留空 tag）')
  let ts = typedTimestamps.value.split('\n').map((s) => s.trim()).filter(Boolean).map(Number)
  if (!ts.length) {
    const now = Date.now()
    ts = vals.map((_, i) => now + i)
  }
  if (ts.length !== vals.length) throw new Error('时间戳行数需与字段值一致')
  if (ts.some((n) => !Number.isSafeInteger(n))) throw new Error('时间戳必须是安全整数（ms）')
  const fieldCol: Record<string, unknown> = {
    name: typedFieldName.value.trim() || 'value',
    type: typedFieldType.value === 'int' ? 2 : 1,
  }
  if (typedFieldType.value === 'int') {
    fieldCol.int64_values = vals.map((v) => {
      if (!/^-?\d+$/.test(v)) throw new Error(`非法整数: ${v}`)
      const n = Number(v)
      if (!Number.isSafeInteger(n)) throw new Error(`整数越界: ${v}`)
      return n
    })
  } else {
    fieldCol.float64_values = vals.map((v) => {
      const n = Number(v)
      if (!Number.isFinite(n)) throw new Error(`非法浮点: ${v}`)
      return n
    })
  }
  const batch: Record<string, unknown> = {
    database: selectedDb.value,
    retention_policy: retentionPolicy.value,
    measurement: typedMeasurement.value.trim(),
    precision: 'ms',
    timestamps: ts,
    fields: [fieldCol],
  }
  if (tags.length && typedTagKey.value.trim()) {
    batch.tags = [{ name: typedTagKey.value.trim(), values: tags }]
  }
  return batch
}

async function submit() {
  loading.value = true
  actionError.value = ''
  result.value = null
  try {
    if (writeMode.value === 'typed') {
      const batch = buildTypedBatch()
      await apiPost('/api/v1/data/write/typed', { batch, options: { sync: syncWrite.value } })
      result.value = { ok: true, message: `TypedBatch 写入成功（${(batch.timestamps as number[]).length} 点）` }
      success(result.value.message)
      return
    }
    let points: Record<string, unknown>[] = []
    if (writeMode.value === 'form') {
      points = buildFormPoints(formRows.value)
    } else if (writeMode.value === 'line') {
      const parsed = parseLineProtocolDetailed(lineInput.value)
      if (parsed.errors.length) {
        const summary = parsed.errors.slice(0, 5).join('；') + (parsed.errors.length > 5 ? ` 等共 ${parsed.errors.length} 处` : '')
        if (!parsed.points.length) throw new Error(summary)
        notifyError(`Line Protocol 部分行无效：${summary}`)
      }
      points = parsed.points
    } else {
      points = parsePrometheusText(lineInput.value)
    }
    if (!points.length) throw new Error('没有可写入的点')
    for (const p of points) {
      p.database = selectedDb.value
      p.retention_policy = retentionPolicy.value
    }
    const writePath = usePointsTyped.value ? '/api/v1/data/write/points-typed' : '/api/v1/data/write'
    await apiPost(writePath, { points, options: { sync: syncWrite.value } })
    result.value = { ok: true, message: `写入成功（${points.length} 点，${writePath}）` }
    success(result.value.message)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '写入失败'
    notifyError(actionError.value)
    result.value = { ok: false, message: actionError.value }
  } finally {
    loading.value = false
  }
}

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
      <label class="text-xs text-slate-500">Database
        <select v-model="selectedDb" class="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800">
          <option v-for="db in databases" :key="db" :value="db">{{ db }}</option>
        </select>
      </label>
      <label class="text-xs text-slate-500">RP
        <select v-model="retentionPolicy" class="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800">
          <option v-for="rp in (retentionPolicies.length?retentionPolicies:['autogen'])" :key="rp" :value="rp">{{ rp }}</option>
        </select>
      </label>
      <label class="flex items-center gap-2 text-xs text-slate-600 md:mt-6">
        <input v-model="syncWrite" type="checkbox" /> Sync
      </label>
      <label v-if="writeMode!=='typed'" class="flex items-center gap-2 text-xs text-slate-600 md:mt-6">
        <input v-model="usePointsTyped" type="checkbox" /> points-typed 路径
      </label>
    </div>

    <div v-if="writeMode==='form'" class="space-y-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900">
      <div v-for="(row, idx) in formRows" :key="idx" class="rounded border border-slate-100 p-3 dark:border-slate-800">
        <div class="mb-2 flex justify-between text-xs font-medium">Row {{ idx+1 }}
          <button class="text-red-500" @click="removeRow(idx)"><Trash2 class="h-3.5 w-3.5" /></button>
        </div>
        <div class="grid gap-2 md:grid-cols-3">
          <input v-model="row.measurement" placeholder="measurement" class="rounded border px-2 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" />
          <input v-model="row.timestamp" placeholder="timestamp ms" class="rounded border px-2 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" />
          <div class="text-[11px] text-slate-400">tags/fields 在下方</div>
        </div>
        <div class="mt-2 grid gap-2 md:grid-cols-2">
          <div>
            <p class="mb-1 text-[11px] text-slate-500">Tags</p>
            <div v-for="(tg, ti) in row.tags" :key="ti" class="mb-1 flex gap-1">
              <input v-model="tg.key" class="w-1/2 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" placeholder="key" />
              <input v-model="tg.value" class="w-1/2 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" placeholder="value" />
            </div>
            <button class="text-[11px] text-slate-500" @click="row.tags.push({key:'',value:''})">+ tag</button>
          </div>
          <div>
            <p class="mb-1 text-[11px] text-slate-500">Fields</p>
            <div v-for="(fd, fi) in row.fields" :key="fi" class="mb-1 flex gap-1">
              <input v-model="fd.key" class="w-1/3 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" placeholder="key" />
              <input v-model="fd.value" class="w-1/3 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800" placeholder="value" />
              <select v-model="fd.type" class="w-1/3 rounded border px-1 py-1 text-xs dark:border-slate-600 dark:bg-slate-800">
                <option v-for="ft in fieldTypes" :key="ft.value" :value="ft.value">{{ ft.label }}</option>
              </select>
            </div>
            <button class="text-[11px] text-slate-500" @click="row.fields.push({key:'',value:'',type:'float'})">+ field</button>
          </div>
        </div>
      </div>
      <button class="inline-flex items-center gap-1 text-xs text-slate-600" @click="addRow"><Plus class="h-3 w-3" /> 添加行</button>
    </div>

    <div v-else-if="writeMode==='typed'" class="grid gap-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900 md:grid-cols-2">
      <p class="md:col-span-2 text-xs text-slate-500">直接构造 TypedBatch 调用 <code>/api/v1/data/write/typed</code>（推荐高性能路径）</p>
      <label class="text-xs">Measurement<input v-model="typedMeasurement" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" /></label>
      <label class="text-xs">Tag key<input v-model="typedTagKey" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" /></label>
      <label class="text-xs">Tag values（每行一个）
        <textarea v-model="typedTagValues" rows="4" class="mt-1 w-full rounded border px-2 py-1.5 font-mono text-xs dark:border-slate-600 dark:bg-slate-800" />
      </label>
      <label class="text-xs">Field values（每行一个）
        <textarea v-model="typedFieldValues" rows="4" class="mt-1 w-full rounded border px-2 py-1.5 font-mono text-xs dark:border-slate-600 dark:bg-slate-800" />
      </label>
      <label class="text-xs">Field name<input v-model="typedFieldName" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" /></label>
      <label class="text-xs">Field type
        <select v-model="typedFieldType" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800">
          <option value="float">float</option>
          <option value="int">int</option>
        </select>
      </label>
      <label class="text-xs md:col-span-2">Timestamps ms（可选，每行一个；空则自动生成）
        <textarea v-model="typedTimestamps" rows="3" class="mt-1 w-full rounded border px-2 py-1.5 font-mono text-xs dark:border-slate-600 dark:bg-slate-800" />
      </label>
    </div>

    <div v-else class="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900">
      <textarea v-model="lineInput" rows="10" class="w-full rounded border border-slate-300 px-3 py-2 font-mono text-xs dark:border-slate-600 dark:bg-slate-800" :placeholder="modeLabel" />
    </div>

    <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900" :disabled="loading" @click="submit">
      <Send class="h-4 w-4" /> {{ loading ? t('loading') : '写入' }}
    </button>
    <p v-if="actionError" class="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>
    <p v-if="result?.ok" class="rounded-xl border border-green-200 bg-green-50 p-3 text-sm text-green-700">{{ result.message }}</p>
  </div>
</template>
