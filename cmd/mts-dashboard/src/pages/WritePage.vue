<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { apiGet, apiPost } from '@/api/client'
import { Send, Plus, Trash2, Database, Clock, ToggleLeft, ToggleRight, Table2, FileText, BarChart3 } from 'lucide-vue-next'
import { fieldTypes, buildFormPoints, parseLineProtocol, parsePrometheusText, type FormRow } from '@/composables/usePointParsers'

type WriteMode = 'form' | 'line' | 'prometheus'

interface DatabaseListResponse { measurements: string[] }

const databases = ref<string[]>([])
const retentionPolicies = ref<string[]>([])
const selectedDb = ref('')
const retentionPolicy = ref('autogen')
const syncWrite = ref(false)
const writeMode = ref<WriteMode>('form')
const lineInput = ref('')
const formRows = ref<FormRow[]>([createEmptyRow()])
const result = ref<{ ok: boolean; message: string } | null>(null)
const loading = ref(false)
const actionError = ref('')

const lineProtocolExample = `cpu,host=server01 usage=75.5,temperature=42 ${Date.now() * 1e6}
cpu,host=server02 usage=60.0,temperature=38 ${Date.now() * 1e6}
mem,host=server01 used=8589934592 ${Date.now() * 1e6}
`

const prometheusExample = `cpu_usage{host="server01",mode="idle"} 75.5 ${Date.now()}
cpu_usage{host="server02",mode="idle"} 60.0 ${Date.now()}
mem_used{host="server01"} 8589934592 ${Date.now()}
`

function createEmptyRow(): FormRow {
  return {
    measurement: '',
    tags: [{ key: '', value: '' }],
    fields: [{ key: 'value', value: '', type: 'float' }],
    timestamp: String(Date.now() * 1e6),
  }
}

function addTag(row: FormRow) { row.tags.push({ key: '', value: '' }) }
function removeTag(row: FormRow, idx: number) { if (row.tags.length > 1) row.tags.splice(idx, 1) }
function addField(row: FormRow) { row.fields.push({ key: '', value: '', type: 'float' }) }
function removeField(row: FormRow, idx: number) { if (row.fields.length > 1) row.fields.splice(idx, 1) }
function addRow() { formRows.value.push(createEmptyRow()) }
function removeRow(idx: number) { if (formRows.value.length > 1) formRows.value.splice(idx, 1) }

onMounted(async () => {
  try {
    const data = await apiGet<DatabaseListResponse>('/api/v1/admin/databases')
    databases.value = (data.measurements ?? []).sort()
    if (databases.value.length) selectedDb.value = databases.value[0]
  } catch (_) { /* 非关键 */ }
})

watch(selectedDb, async (db) => {
  retentionPolicies.value = []
  retentionPolicy.value = 'autogen'
  if (!db) return
  try {
    const data = await apiGet<{ policies: { name: string }[] }>(`/api/v1/admin/databases/${encodeURIComponent(db)}/retention-policies`)
    retentionPolicies.value = (data.policies ?? []).map((p) => p.name)
  } catch (_) { /* 非关键 */ }
})

async function doWrite() {
  actionError.value = ''
  result.value = null
  loading.value = true
  try {
    const options: Record<string, unknown> = { sync: syncWrite.value }
    let points: Record<string, unknown>[]
    if (writeMode.value === 'form') {
      points = buildFormPoints(formRows.value)
    } else if (writeMode.value === 'prometheus') {
      points = parsePrometheusText(lineInput.value)
    } else {
      points = parseLineProtocol(lineInput.value)
    }
    if (!points.length) {
      actionError.value = '未能解析任何有效数据'
      loading.value = false
      return
    }
    for (const p of points) {
      (p as Record<string, unknown>).database = selectedDb.value
      ;(p as Record<string, unknown>).retention_policy = retentionPolicy.value
    }
    await apiPost('/api/v1/data/write', { points, options })
    result.value = { ok: true, message: `写入成功 (${points.length} 条)` }
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '写入失败'
  } finally {
    loading.value = false
  }
}

function setNow(row: FormRow) { row.timestamp = String(Date.now() * 1e6) }
</script>

<template>
  <div class="space-y-6">
    <p v-if="actionError" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ actionError }}</p>

    <!-- 目标配置 -->
    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div class="border-b border-slate-100 bg-slate-50/50 px-6 py-4">
        <h3 class="text-sm font-semibold text-slate-800">目标配置</h3>
      </div>
      <div class="grid grid-cols-1 gap-4 p-6 sm:grid-cols-4">
        <div>
          <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500">
            <Database class="h-3 w-3" />目标数据库
          </label>
          <select v-model="selectedDb" class="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100">
            <option v-if="!databases.length" value="" disabled>无数据</option>
            <option v-for="db in databases" :key="db" :value="db">{{ db }}</option>
          </select>
        </div>
        <div>
          <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500">
            <Clock class="h-3 w-3" />保留策略
          </label>
          <select v-model="retentionPolicy" class="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100">
            <option v-for="rp in retentionPolicies" :key="rp" :value="rp">{{ rp }}</option>
            <option v-if="!retentionPolicies.length" value="autogen">autogen (默认)</option>
          </select>
        </div>
        <div class="flex items-end pb-1">
          <label class="flex cursor-pointer select-none items-center gap-3 rounded-lg border border-slate-200 bg-white px-4 py-2.5 shadow-sm transition hover:border-slate-300">
            <div class="flex flex-col">
              <span class="text-sm font-medium text-slate-700">Sync 写入</span>
              <span class="text-xs text-slate-400">WAL 同步落盘</span>
            </div>
            <div class="relative ml-auto">
              <input type="checkbox" v-model="syncWrite" class="sr-only" />
              <component :is="syncWrite ? ToggleRight : ToggleLeft" :class="syncWrite ? 'text-slate-800' : 'text-slate-300'" class="h-6 w-6 transition" />
            </div>
          </label>
        </div>
      </div>
    </div>

    <!-- 数据格式 -->
    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 bg-slate-50/50 px-6 py-3">
        <div class="flex items-center gap-1 rounded-lg bg-slate-100 p-0.5">
          <button
            v-for="opt in ([
              { v: 'form' as const, label: '表单', icon: Table2 },
              { v: 'line' as const, label: '行协议', icon: FileText },
              { v: 'prometheus' as const, label: 'Prometheus', icon: BarChart3 },
            ])"
            :key="opt.v"
            :class="[
              'flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition',
              writeMode === opt.v
                ? 'bg-white text-slate-800 shadow-sm'
                : 'text-slate-500 hover:text-slate-700',
            ]"
            @click="writeMode = opt.v"
          >
            <component :is="opt.icon" class="h-3.5 w-3.5" />
            {{ opt.label }}
          </button>
        </div>
        <button
          v-if="writeMode !== 'form'"
          class="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs text-slate-600 shadow-sm hover:bg-slate-50"
          @click="lineInput = writeMode === 'prometheus' ? prometheusExample : lineProtocolExample"
        >
          填充示例
        </button>
      </div>

      <!-- 表单模式 -->
      <div v-if="writeMode === 'form'" class="space-y-4 p-6">
        <div
          v-for="(row, ri) in formRows"
          :key="ri"
          class="group rounded-xl border border-slate-200 bg-slate-50/30 p-5"
        >
          <div class="mb-4 flex items-center justify-between">
            <span class="text-xs font-semibold uppercase tracking-wider text-slate-400">数据行 #{{ ri + 1 }}</span>
            <button
              v-if="formRows.length > 1"
              class="rounded-lg p-1 text-slate-300 opacity-0 transition hover:bg-red-50 hover:text-red-500 group-hover:opacity-100"
              @click="removeRow(ri)"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </div>

          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label class="mb-1.5 block text-xs font-medium text-slate-500">Measurement</label>
              <input v-model="row.measurement" placeholder="例如 cpu" class="w-full rounded-lg border border-slate-200 bg-white px-3 py-2.5 text-sm shadow-sm transition placeholder:text-slate-300 focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-slate-500">Timestamp (纳秒)</label>
              <div class="flex gap-2">
                <input v-model="row.timestamp" type="number" class="flex-1 rounded-lg border border-slate-200 bg-white px-3 py-2.5 text-sm shadow-sm transition focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100" />
                <button class="rounded-lg border border-slate-200 bg-white px-3 py-2.5 text-sm text-slate-500 shadow-sm transition hover:bg-slate-50 hover:text-slate-700" @click="setNow(row)">now</button>
              </div>
            </div>
          </div>

          <div class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div>
              <label class="mb-2 block text-xs font-medium text-slate-500">Tags</label>
              <div class="space-y-2">
                <div v-for="(tag, ti) in row.tags" :key="ti" class="flex items-center gap-2">
                  <input v-model="tag.key" placeholder="key" class="w-24 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm shadow-sm transition placeholder:text-slate-300 focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100" />
                  <span class="text-sm font-medium text-slate-300">=</span>
                  <input v-model="tag.value" placeholder="value" class="flex-1 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm shadow-sm transition placeholder:text-slate-300 focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100" />
                  <button v-if="row.tags.length > 1" class="rounded-lg p-1.5 text-slate-300 opacity-0 transition hover:bg-red-50 hover:text-red-500 group-hover:opacity-100" @click="removeTag(row, ti)"><Trash2 class="h-3.5 w-3.5" /></button>
                </div>
              </div>
              <button class="mt-2 text-xs font-medium text-slate-400 transition hover:text-slate-600" @click="addTag(row)"><Plus class="inline h-3 w-3" /> 添加 Tag</button>
            </div>
            <div>
              <label class="mb-2 block text-xs font-medium text-slate-500">Fields</label>
              <div class="space-y-2">
                <div v-for="(field, fi) in row.fields" :key="fi" class="flex items-center gap-2">
                  <input v-model="field.key" placeholder="key" class="w-24 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm shadow-sm transition placeholder:text-slate-300 focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100" />
                  <select v-model="field.type" class="w-20 rounded-lg border border-slate-200 bg-white px-2 py-2 text-sm shadow-sm transition focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100">
                    <option v-for="ft in fieldTypes" :key="ft.value" :value="ft.value">{{ ft.label }}</option>
                  </select>
                  <input v-model="field.value" placeholder="值" class="flex-1 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm shadow-sm transition placeholder:text-slate-300 focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100" />
                  <button v-if="row.fields.length > 1" class="rounded-lg p-1.5 text-slate-300 opacity-0 transition hover:bg-red-50 hover:text-red-500 group-hover:opacity-100" @click="removeField(row, fi)"><Trash2 class="h-3.5 w-3.5" /></button>
                </div>
              </div>
              <button class="mt-2 text-xs font-medium text-slate-400 transition hover:text-slate-600" @click="addField(row)"><Plus class="inline h-3 w-3" /> 添加 Field</button>
            </div>
          </div>
        </div>

        <button class="flex w-full items-center justify-center gap-2 rounded-xl border-2 border-dashed border-slate-200 py-4 text-sm font-medium text-slate-400 transition hover:border-slate-300 hover:text-slate-600" @click="addRow">
          <Plus class="h-4 w-4" />添加数据行
        </button>
        <p class="text-center text-xs text-slate-400">表单自动转换为 Points 格式，注入上方选择的数据库和保留策略。</p>
      </div>

      <!-- 文本协议模式 -->
      <div v-else class="p-6">
        <p class="mb-3 text-xs text-slate-400">
          <template v-if="writeMode === 'line'">
            每行一条数据，格式：<code class="rounded bg-slate-100 px-1.5 py-0.5 text-[11px] text-slate-600">measurement,tag=value field=value [timestamp]</code>
          </template>
          <template v-else>
            每行一条数据，格式：<code class="rounded bg-slate-100 px-1.5 py-0.5 text-[11px] text-slate-600">metric{label="value"} value [timestamp_ms]</code>
          </template>
        </p>
        <textarea
          v-model="lineInput"
          :placeholder="writeMode === 'line' ? 'cpu,host=server01 usage=75.5 1234567890000000000' : 'cpu_usage{host=server01} 75.5 1234567890'"
          class="h-64 w-full resize-y rounded-xl border border-slate-200 bg-slate-950 p-5 font-mono text-sm leading-relaxed text-emerald-400 shadow-sm transition placeholder:text-slate-600 focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100"
          spellcheck="false"
        />
      </div>
    </div>

    <!-- 底部操作栏 -->
    <div class="flex items-center justify-between rounded-2xl border border-slate-200 bg-white px-6 py-4 shadow-sm">
      <div v-if="result" :class="result.ok ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : 'border-red-200 bg-red-50 text-red-700'" class="rounded-xl border px-4 py-2 text-sm font-medium">
        {{ result.message }}
      </div>
      <div v-else />
      <button
        :disabled="loading"
        class="flex items-center gap-2 rounded-xl bg-slate-800 px-6 py-2.5 text-sm font-medium text-white shadow-sm transition hover:bg-slate-700 disabled:opacity-40"
        @click="doWrite"
      >
        <Send class="h-4 w-4" />
        {{ loading ? '写入中...' : '执行写入' }}
      </button>
    </div>
  </div>
</template>
