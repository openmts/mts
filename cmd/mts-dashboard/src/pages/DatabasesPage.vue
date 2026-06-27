<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { apiGet, apiPost, apiDelete } from '@/api/client'
import {
  Plus, Trash2, ChevronDown, ChevronRight, Table2, Tag, Clock,
} from 'lucide-vue-next'

interface MeasurementResponse { measurements: string[] }
interface RetentionPolicy { name: string; duration: number }
interface RetentionPoliciesResponse { policies: RetentionPolicy[] }
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
  retentionPolicies: RetentionPolicy[]
  newRpName: string
  newRpDuration: string
}

const databases = ref<DatabaseEntry[]>([])
const newDbName = ref('')
const loadError = ref('')
const actionError = ref('')

onMounted(async () => {
  try {
    const data = await apiGet<MeasurementResponse>('/api/v1/admin/databases')
    databases.value = (data.measurements ?? []).map((name) => ({
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
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
})

async function toggleExpand(db: DatabaseEntry) {
  db.expanded = !db.expanded
  if (db.expanded && !db.loaded) {
    db.loading = true
    db.loaded = true
    try {
      const [measData, rpData] = await Promise.all([
        apiGet<MeasurementResponse>(`/api/v1/data/databases/${encodeURIComponent(db.name)}/measurements`),
        apiGet<RetentionPoliciesResponse>(`/api/v1/admin/databases/${encodeURIComponent(db.name)}/retention-policies`),
      ])
      db.measurements = (measData.measurements ?? []).map((m) => ({
        name: m,
        expanded: false,
        loading: false,
        fields: [],
        series: [],
      }))
      db.retentionPolicies = rpData.policies ?? []
    } catch (e) {
      actionError.value = e instanceof Error ? e.message : '加载详情失败'
      db.loaded = false
    } finally {
      db.loading = false
    }
  }
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
      actionError.value = e instanceof Error ? e.message : '加载元数据失败'
    } finally {
      meas.loading = false
    }
  }
}

async function createDatabase() {
  if (!newDbName.value.trim()) return
  actionError.value = ''
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
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建失败'
  }
}

async function deleteDatabase(name: string) {
  const input = prompt(`请输入数据库名称 "${name}" 以确认删除：`)
  if (input !== name) return
  actionError.value = ''
  try {
    await apiDelete(`/api/v1/admin/databases/${encodeURIComponent(name)}`)
    databases.value = databases.value.filter((d) => d.name !== name)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
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
    actionError.value = '无效的 duration 格式 (如 24h, 7d, 30m)'
    return
  }
  actionError.value = ''
  try {
    await apiPost(`/api/v1/admin/databases/${encodeURIComponent(db.name)}/retention-policies`, {
      policy: { name, duration: durationNs },
    })
    db.retentionPolicies.push({ name, duration: durationNs })
    db.newRpName = ''
    db.newRpDuration = ''
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建保留策略失败'
  }
}

function parseDuration(s: string): number {
  const match = s.match(/^(\d+)(h|m|s|d)$/)
  if (!match) throw new Error('invalid')
  const v = parseInt(match[1])
  switch (match[2]) {
    case 's': return v * 1e9
    case 'm': return v * 60 * 1e9
    case 'h': return v * 3600 * 1e9
    case 'd': return v * 86400 * 1e9
    default: throw new Error('invalid')
  }
}

function formatDuration(ns: number): string {
  if (ns >= 86400e9) return (ns / 86400e9).toFixed(0) + 'd'
  if (ns >= 3600e9) return (ns / 3600e9).toFixed(0) + 'h'
  if (ns >= 60e9) return (ns / 60e9).toFixed(0) + 'm'
  return (ns / 1e9).toFixed(0) + 's'
}

function fieldTypeLabel(t: number): string {
  const types = ['', 'float', 'int', 'string', 'bool']
  return types[t] ?? `type_${t}`
}

function seriesTagPairs(tags: Record<string, string> | null | undefined): string {
  if (!tags) return ''
  return Object.entries(tags).map(([k, v]) => `${k}=${v}`).join(', ')
}
</script>

<template>
  <div>
    <p v-if="loadError" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ loadError }}</p>
    <p v-if="actionError" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>

    <div class="mb-4 flex gap-2">
      <input v-model="newDbName" type="text" placeholder="新数据库名称" class="w-64 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none focus:ring-1 focus:ring-slate-500" @keyup.enter="createDatabase" />
      <button class="flex items-center gap-1.5 whitespace-nowrap rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="createDatabase">
        <Plus class="h-4 w-4 shrink-0" />创建
      </button>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white">
      <div v-if="!databases.length" class="p-6 text-center text-sm text-slate-400">暂无数据库</div>
      <div v-for="db in databases" :key="db.name" class="border-b border-slate-100 last:border-b-0">
        <!-- 数据库行 -->
        <div class="flex items-center justify-between px-4 py-3 hover:bg-slate-50 cursor-pointer" @click="toggleExpand(db)">
          <div class="flex items-center gap-2">
            <component :is="db.expanded ? ChevronDown : ChevronRight" class="h-4 w-4 text-slate-400" />
            <Table2 class="h-4 w-4 text-slate-400" />
            <span class="text-sm font-medium text-slate-700">{{ db.name }}</span>
            <span v-if="db.measurements.length" class="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500">{{ db.measurements.length }} 个 measurement</span>
          </div>
          <button class="rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-600" @click.stop="deleteDatabase(db.name)">
            <Trash2 class="h-4 w-4" />
          </button>
        </div>

        <!-- 展开内容 -->
        <div v-if="db.expanded" class="border-t border-slate-100 bg-slate-50">
          <p v-if="db.loading" class="px-10 py-4 text-sm text-slate-400">加载中...</p>
          <template v-else>
            <!-- Measurements -->
            <div class="px-6 py-3">
              <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">Measurements</p>
              <div v-if="!db.measurements.length" class="text-xs text-slate-400">暂无（写入数据后自动创建）</div>
              <div v-for="meas in db.measurements" :key="meas.name" class="mb-1 rounded-lg border border-slate-200 bg-white">
                <div class="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-slate-50" @click="toggleMeasurement(meas, db.name)">
                  <component :is="meas.expanded ? ChevronDown : ChevronRight" class="h-3.5 w-3.5 text-slate-400" />
                  <Tag class="h-3.5 w-3.5 text-slate-400" />
                  <span class="text-sm text-slate-700">{{ meas.name }}</span>
                </div>
                <div v-if="meas.expanded" class="border-t border-slate-100 px-4 py-2">
                  <p v-if="meas.loading" class="text-xs text-slate-400">加载中...</p>
                  <template v-else>
                    <!-- Fields -->
                    <div class="mb-3">
                      <p class="mb-1 text-xs font-medium text-slate-500">Fields</p>
                      <table v-if="meas.fields.length" class="w-full text-xs">
                        <thead><tr class="text-left text-slate-400"><th class="pb-1 pr-3 font-normal">名称</th><th class="pb-1 font-normal">类型</th></tr></thead>
                        <tbody>
                          <tr v-for="f in meas.fields" :key="f.name" class="border-t border-slate-100">
                            <td class="py-1 pr-3 font-mono text-slate-700">{{ f.name }}</td>
                            <td class="py-1 text-slate-500">{{ fieldTypeLabel(f.type) }}</td>
                          </tr>
                        </tbody>
                      </table>
                      <p v-else class="text-xs text-slate-400">暂无字段</p>
                    </div>
                    <!-- Series -->
                    <div>
                      <p class="mb-1 text-xs font-medium text-slate-500">Series ({{ meas.series.length }})</p>
                      <div v-if="meas.series.length" class="max-h-32 overflow-auto">
                        <table class="w-full text-xs">
                          <thead><tr class="text-left text-slate-400"><th class="pb-1 pr-3 font-normal">ID</th><th class="pb-1 font-normal">Tags</th></tr></thead>
                          <tbody>
                            <tr v-for="s in meas.series" :key="s.id" class="border-t border-slate-100">
                              <td class="py-1 pr-3 font-mono text-slate-600">{{ s.id }}</td>
                              <td class="py-1 text-slate-500">{{ seriesTagPairs(s.tags) || '-' }}</td>
                            </tr>
                          </tbody>
                        </table>
                      </div>
                      <p v-else class="text-xs text-slate-400">暂无 series</p>
                    </div>
                  </template>
                </div>
              </div>
            </div>

            <!-- 保留策略 -->
            <div class="border-t border-slate-200 px-6 py-3">
              <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">保留策略</p>
              <div v-if="db.retentionPolicies.length" class="mb-3 space-y-1">
                <div v-for="rp in db.retentionPolicies" :key="rp.name" class="flex items-center gap-2 rounded bg-white border border-slate-200 px-3 py-1.5">
                  <Clock class="h-3.5 w-3.5 text-slate-400" />
                  <span class="text-sm text-slate-700 font-medium">{{ rp.name }}</span>
                  <span class="text-xs text-slate-500">{{ formatDuration(rp.duration) }}</span>
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <input v-model="db.newRpName" type="text" placeholder="策略名称" class="w-28 rounded border border-slate-300 px-2 py-1 text-xs focus:border-slate-500 focus:outline-none" />
                <input v-model="db.newRpDuration" type="text" placeholder="时长 (24h/7d)" class="w-24 rounded border border-slate-300 px-2 py-1 text-xs focus:border-slate-500 focus:outline-none" />
                <button class="flex items-center gap-1 whitespace-nowrap rounded bg-slate-800 px-3 py-1 text-xs font-medium text-white hover:bg-slate-700" @click="createRetentionPolicy(db)">
                  <Plus class="h-3.5 w-3.5 shrink-0" />添加
                </button>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
