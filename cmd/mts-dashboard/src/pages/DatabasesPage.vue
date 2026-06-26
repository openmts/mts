<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost, apiDelete } from '@/api/client'
import { Plus, Trash2, ChevronDown, ChevronRight } from 'lucide-vue-next'

interface MeasurementResponse { measurements: string[] }
interface RetentionPolicy { name: string; duration: number }
interface RetentionPoliciesResponse { policies: RetentionPolicy[] }

interface DatabaseEntry {
  name: string
  measurements: string[]
  retentionPolicies: RetentionPolicy[]
  expanded: boolean
  loading: boolean
}

const databases = ref<DatabaseEntry[]>([])
const newDbName = ref('')
const loadError = ref('')
const actionError = ref('')

onMounted(async () => {
  try {
    const data = await apiGet<MeasurementResponse>('/api/v1/data/databases/')
    databases.value = (data.measurements ?? []).map((name) => ({
      name,
      measurements: [],
      retentionPolicies: [],
      expanded: false,
      loading: false,
    }))
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
})

async function toggleExpand(db: DatabaseEntry) {
  db.expanded = !db.expanded
  if (db.expanded && !db.measurements.length && !db.retentionPolicies.length) {
    db.loading = true
    try {
      const [measData, rpData] = await Promise.all([
        apiGet<MeasurementResponse>(`/api/v1/data/databases/${db.name}/measurements`),
        apiGet<RetentionPoliciesResponse>(`/api/v1/admin/databases/${db.name}/retention-policies`),
      ])
      db.measurements = measData.measurements ?? []
      db.retentionPolicies = rpData.policies ?? []
    } catch (e) {
      actionError.value = e instanceof Error ? e.message : '加载详情失败'
    } finally {
      db.loading = false
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
    })
    newDbName.value = ''
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建失败'
  }
}

async function deleteDatabase(name: string) {
  if (!confirm(`确定删除数据库 ${ name }？此操作不可逆。`)) return
  actionError.value = ''
  try {
    await apiDelete(`/api/v1/admin/databases/${name}`)
    databases.value = databases.value.filter((d) => d.name !== name)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
  }
}
</script>

<template>
  <div>
    <p v-if="loadError" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ loadError }}</p>
    <p v-if="actionError" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>
    <div class="mb-4 flex gap-2">
      <input v-model="newDbName" type="text" placeholder="新数据库名称" class="w-64 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none focus:ring-1 focus:ring-slate-500" @keyup.enter="createDatabase" />
      <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="createDatabase">
        <Plus class="h-4 w-4" /> 创建
      </button>
    </div>
    <div class="rounded-xl border border-slate-200 bg-white">
      <div v-if="!databases.length" class="p-6 text-center text-sm text-slate-400">暂无数据库</div>
      <div v-for="db in databases" :key="db.name" class="border-b border-slate-100 last:border-b-0">
        <div class="flex items-center justify-between px-4 py-3 hover:bg-slate-50 cursor-pointer" @click="toggleExpand(db)">
          <div class="flex items-center gap-2">
            <component :is="db.expanded ? ChevronDown : ChevronRight" class="h-4 w-4 text-slate-400" />
            <span class="text-sm font-medium text-slate-700">{{ db.name }}</span>
          </div>
          <button class="rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-600" @click.stop="deleteDatabase(db.name)">
            <Trash2 class="h-4 w-4" />
          </button>
        </div>
        <div v-if="db.expanded" class="border-t border-slate-100 bg-slate-50 px-10 py-3">
          <p v-if="db.loading" class="text-sm text-slate-400">加载中...</p>
          <template v-else>
            <div v-if="db.measurements.length" class="mb-2">
              <p class="mb-1 text-xs font-medium text-slate-500">Measurements</p>
              <div class="flex flex-wrap gap-1">
                <span v-for="m in db.measurements" :key="m" class="rounded bg-slate-200 px-2 py-0.5 text-xs text-slate-600">{{ m }}</span>
              </div>
            </div>
            <div v-if="db.retentionPolicies.length">
              <p class="mb-1 text-xs font-medium text-slate-500">保留策略</p>
              <div class="flex flex-wrap gap-1">
                <span v-for="rp in db.retentionPolicies" :key="rp.name" class="rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-700">{{ rp.name }} ({{ (rp.duration / 1e9 / 3600).toFixed(0) }}h)</span>
              </div>
            </div>
            <p v-if="!db.measurements.length && !db.retentionPolicies.length" class="text-sm text-slate-400">暂无详情</p>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
