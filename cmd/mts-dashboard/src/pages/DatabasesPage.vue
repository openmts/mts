<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost, apiDelete } from '@/api/client'
import { listDatabases, listMeasurements, listRetentionPolicies } from '@/api/meta'
import {
  Plus, Trash2, ChevronDown, ChevronRight, Table2, Tag, Clock,
} from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { useI18n } from '@/composables/useI18n'
interface FieldSchema { measurement: string; name: string; type: number }
interface FieldsResponse { fields: FieldSchema[] }
interface Series { id: number; measurement: string; tags: Record<string, string> }
interface SeriesResponse { series: Series[] }
interface MeasurementEntry {
  name: string
  expanded: boolean
  loading: boolean
  fields: FieldSchema[]
  series: Series[]
}
interface DatabaseEntry {
  name: string
  expanded: boolean
  loading: boolean
  loaded: boolean
  measurements: MeasurementEntry[]
  retentionPolicies: { name: string; duration: number }[]
  newRpName: string
  newRpDuration: string
}
const { isAdmin } = useAuth()
const { t } = useI18n()
const { success, error: notifyError } = useNotify()
const databases = ref<DatabaseEntry[]>([])
const newDbName = ref('')
const loadError = ref('')
const actionResult = ref<ActionResult | null>(null)
const confirmOpen = ref(false)
const confirmDbName = ref('')
const confirmLoading = ref(false)
onMounted(async () => {
  if (!isAdmin.value) return
  try {
    const names = await listDatabases()
    databases.value = names.map((name) => ({
      name,
      expanded: false,
      loading: false,
      loaded: false,
      measurements: [],
      retentionPolicies: [],
      newRpName: '',
      newRpDuration: '',
    }))
  } catch (e) {
    loadError.value = formatCaughtError(e)
  }
})
async function loadDatabaseDetails(db: DatabaseEntry) {
  db.loading = true
  actionResult.value = null
  try {
    const [meas, rps] = await Promise.all([
      listMeasurements(db.name),
      listRetentionPolicies(db.name),
    ])
    db.measurements = meas.map((m) => ({
      name: m,
      expanded: false,
      loading: false,
      fields: [],
      series: [],
    }))
    db.retentionPolicies = rps.map((p) => ({ name: p.name, duration: p.duration ?? 0 }))
    db.loaded = true
  } catch (e) {
    const msg = formatCaughtError(e); actionResult.value = makeActionResult('error', msg)
    db.loaded = false
    db.expanded = false
  } finally {
    db.loading = false
  }
}
async function toggleExpand(db: DatabaseEntry) {
  if (db.expanded) {
    db.expanded = false
    return
  }
  db.expanded = true
  if (!db.loaded) await loadDatabaseDetails(db)
}
async function toggleMeasurement(meas: MeasurementEntry, dbName: string) {
  meas.expanded = !meas.expanded
  if (meas.expanded && !meas.fields.length) {
    meas.loading = true
    try {
      const [fieldsData, seriesData] = await Promise.all([
        apiGet<FieldsResponse>(`/api/v1/data/databases/${encodeURIComponent(dbName)}/measurements/${encodeURIComponent(meas.name)}/fields`),
        apiGet<SeriesResponse>(`/api/v1/data/databases/${encodeURIComponent(dbName)}/measurements/${encodeURIComponent(meas.name)}/series`),
      ])
      meas.fields = fieldsData.fields ?? []
      meas.series = seriesData.series ?? []
    } catch (e) {
      const msg = formatCaughtError(e); actionResult.value = makeActionResult('error', msg)
    } finally {
      meas.loading = false
    }
  }
}
async function createDatabase() {
  if (!newDbName.value.trim()) return
  actionResult.value = null
  try {
    await apiPost('/api/v1/admin/databases', { name: newDbName.value.trim() })
    databases.value.push({
      name: newDbName.value.trim(),
      measurements: [],
      retentionPolicies: [],
      expanded: false,
      loading: false,
      loaded: false,
      newRpName: '',
      newRpDuration: '',
    })
    newDbName.value = ''
    actionResult.value = makeActionResult('ok', '数据库已创建')
    success('数据库已创建')
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}
function requestDeleteDatabase(name: string) {
  confirmDbName.value = name
  confirmOpen.value = true
}
async function confirmDeleteDatabase() {
  const name = confirmDbName.value
  if (!name) return
  confirmLoading.value = true
  actionResult.value = null
  try {
    await apiDelete(`/api/v1/admin/databases/${encodeURIComponent(name)}`)
    databases.value = databases.value.filter((d) => d.name !== name)
    confirmOpen.value = false
    success(`数据库 ${name} 已删除`)
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  } finally {
    confirmLoading.value = false
  }
}
async function createRetentionPolicy(db: DatabaseEntry) {
  const name = db.newRpName.trim()
  if (!name) return
  const dur = db.newRpDuration.trim()
  if (!dur) return
  let durationNs: number
  try {
    durationNs = parseDuration(dur)
  } catch {
    const msg = '无效的 duration 格式 (如 24h, 7d, 30m)'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  actionResult.value = null
  try {
    await apiPost(`/api/v1/admin/databases/${encodeURIComponent(db.name)}/retention-policies`, {
      policy: { name, duration: durationNs },
    })
    db.retentionPolicies.push({ name, duration: durationNs })
    db.newRpName = ''
    db.newRpDuration = ''
    actionResult.value = makeActionResult('ok', '保留策略已创建')
    success('保留策略已创建')
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}
function parseDuration(s: string): number {
  const m = s.trim().toLowerCase().match(/^(\d+)(ns|us|ms|s|m|h|d)$/)
  if (!m) throw new Error('bad duration')
  const n = Number(m[1])
  const mul: Record<string, number> = { ns: 1, us: 1e3, ms: 1e6, s: 1e9, m: 60e9, h: 3600e9, d: 86400e9 }
  const v = n * mul[m[2]]
  if (!Number.isSafeInteger(v)) throw new Error('overflow')
  return v
}
function formatDuration(ns: number): string {
  if (!ns) return '0'
  if (ns % 86400e9 === 0) return `${ns / 86400e9}d`
  if (ns % 3600e9 === 0) return `${ns / 3600e9}h`
  if (ns % 60e9 === 0) return `${ns / 60e9}m`
  if (ns % 1e9 === 0) return `${ns / 1e9}s`
  return `${ns}ns`
}
function fieldTypeName(t: number): string {
  return ({ 1: 'float', 2: 'int', 3: 'string', 4: 'bool' } as Record<number, string>)[t] || String(t)
}
</script>
<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-4">
    <ActionResultBanner v-if="loadError" kind="error" :message="loadError" @dismiss="loadError = ''" />
    <ActionResultBanner :result="actionResult" @dismiss="actionResult = null" />
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">{{ t('databases') }}</h1>
        <p class="text-xs mts-muted">管理数据库、measurement 元数据与保留策略</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <input v-model="newDbName" type="text" placeholder="新数据库名称" class="w-56 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" @keyup.enter="createDatabase" />
        <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="createDatabase">
          <Plus class="h-4 w-4" /> 创建数据库
        </button>
      </div>
    </div>

    <div v-if="!databases.length" class="mts-card">
      <EmptyState
        title="暂无数据库"
        description="创建首个数据库后，可在此展开查看 measurement、字段类型与保留策略。"
      >
        <template #action>
          <button type="button" class="mts-btn-primary" @click="createDatabase">创建数据库</button>
        </template>
      </EmptyState>
    </div>

    <div v-else class="space-y-2">
      <div v-for="db in databases" :key="db.name" class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
        <div class="flex items-center justify-between px-4 py-3 hover:bg-slate-50 dark:hover:bg-slate-800">
          <button class="flex items-center gap-2 text-left" @click="toggleExpand(db)">
            <component :is="db.expanded ? ChevronDown : ChevronRight" class="h-4 w-4 text-slate-400 dark:text-slate-500" />
            <span class="text-sm font-medium text-slate-800 dark:text-slate-100">{{ db.name }}</span>
            <span v-if="db.loading" class="text-xs text-slate-400 dark:text-slate-500">加载中…</span>
          </button>
          <button class="rounded p-1 text-slate-400 dark:text-slate-500 hover:text-red-600 dark:text-red-300" title="删除数据库" @click="requestDeleteDatabase(db.name)">
            <Trash2 class="h-4 w-4" />
          </button>
        </div>
        <div v-if="db.expanded && db.loaded" class="border-t border-slate-100">
          <div class="px-6 py-3">
            <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400 dark:text-slate-500">Measurements</p>
            <EmptyState v-if="!db.measurements.length" compact title="暂无 measurement" description="写入数据后将自动出现。" />
            <div v-for="meas in db.measurements" :key="meas.name" class="mb-2 rounded border border-slate-100">
              <button class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800" @click="toggleMeasurement(meas, db.name)">
                <component :is="meas.expanded ? ChevronDown : ChevronRight" class="h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
                <Table2 class="h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
                {{ meas.name }}
              </button>
              <div v-if="meas.expanded" class="border-t border-slate-50 px-4 py-2 text-xs text-slate-600 dark:text-slate-300">
                <div v-if="meas.loading" class="text-slate-400 dark:text-slate-500">加载中…</div>
                <template v-else>
                  <p class="mb-1 font-medium text-slate-500 dark:text-slate-400 dark:text-slate-500">Fields</p>
                  <div class="mb-2 flex flex-wrap gap-1">
                    <span v-for="f in meas.fields" :key="f.name" class="rounded bg-slate-100 dark:bg-slate-800 px-2 py-0.5">{{ f.name }}:{{ fieldTypeName(f.type) }}</span>
                    <span v-if="!meas.fields.length" class="text-slate-400 dark:text-slate-500">无</span>
                  </div>
                  <p class="mb-1 font-medium text-slate-500 dark:text-slate-400 dark:text-slate-500">Series</p>
                  <div class="space-y-1">
                    <div v-for="s in meas.series" :key="s.id" class="flex items-center gap-1">
                      <Tag class="h-3 w-3 text-slate-400 dark:text-slate-500" />
                      <span class="font-mono">{{ s.tags && Object.keys(s.tags).length ? JSON.stringify(s.tags) : '{}' }}</span>
                    </div>
                    <span v-if="!meas.series.length" class="text-slate-400 dark:text-slate-500">无</span>
                  </div>
                </template>
              </div>
            </div>
          </div>
          <div class="border-t border-slate-200 dark:border-slate-700 px-6 py-3">
            <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400 dark:text-slate-500">保留策略</p>
            <div v-if="db.retentionPolicies.length" class="mb-3 space-y-1">
              <div v-for="rp in db.retentionPolicies" :key="rp.name" class="flex items-center gap-2 rounded border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900 px-3 py-1.5">
                <Clock class="h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
                <span class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ rp.name }}</span>
                <span class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ formatDuration(rp.duration) }}</span>
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <input v-model="db.newRpName" type="text" placeholder="策略名称" class="w-28 rounded border border-slate-300 dark:border-slate-600 px-2 py-1 text-xs" />
              <input v-model="db.newRpDuration" type="text" placeholder="时长 (24h/7d)" class="w-24 rounded border border-slate-300 dark:border-slate-600 px-2 py-1 text-xs" />
              <button class="inline-flex items-center gap-1 rounded bg-slate-800 px-3 py-1 text-xs font-medium text-white" @click="createRetentionPolicy(db)">
                <Plus class="h-3.5 w-3.5" />添加
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
    <ConfirmDialog
      v-model:open="confirmOpen"
      title="删除数据库"
      message="此操作不可恢复。请输入数据库名称确认删除。"
      :require-text="confirmDbName"
      confirm-label="删除"
      danger
      :loading="confirmLoading"
      @confirm="confirmDeleteDatabase"
    />
  </div>
</template>
