<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { apiGet, apiPost, apiDelete } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import { Plus, Trash2, Play, Pause, RefreshCw } from 'lucide-vue-next'

interface DownsamplePolicy {
  name: string
  source_database: string
  source_measurement: string
  target_database: string
  target_measurement: string
  interval: number
  functions: { function: string; field: string; as: string }[]
  group_by_tags: string[]
  enabled: boolean
}
interface DownsampleStatus { policy_name: string; completed_until_unix: number; last_run_unix: number; last_error: string }
interface PoliciesResponse { policies: DownsamplePolicy[] }
interface StatusesResponse { statuses: DownsampleStatus[] }

const { isAdmin } = useAuth()
const policies = ref<DownsamplePolicy[]>([])
const statuses = ref<DownsampleStatus[]>([])
const loadError = ref('')
const actionError = ref('')
const showCreate = ref(false)
const newPolicy = ref<DownsamplePolicy>({
  name: '', source_database: '', source_measurement: '',
  target_database: '', target_measurement: '',
  interval: 60000000000, functions: [{ function: 'mean', field: 'value', as: 'mean_value' }],
  group_by_tags: [], enabled: true,
})

onMounted(loadData)

async function loadData() {
  try {
    const [polData, statData] = await Promise.all([
      apiGet<PoliciesResponse>('/api/v1/admin/downsample/policies'),
      apiGet<StatusesResponse>('/api/v1/admin/downsample/statuses'),
    ])
    policies.value = polData.policies ?? []
    statuses.value = statData.statuses ?? []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
}

async function createPolicy() {
  if (!newPolicy.value.name.trim()) return
  actionError.value = ''
  try {
    await apiPost('/api/v1/admin/downsample/policies', newPolicy.value)
    showCreate.value = false
    newPolicy.value = {
      name: '', source_database: '', source_measurement: '',
      target_database: '', target_measurement: '',
      interval: 60000000000, functions: [{ function: 'mean', field: 'value', as: 'mean_value' }],
      group_by_tags: [], enabled: true,
    }
    await loadData()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建失败'
  }
}

const newPolicyTagsText = computed({
  get: () => newPolicy.value.group_by_tags.join(', '),
  set: (v: string) => {
    newPolicy.value.group_by_tags = v.split(',').map((t) => t.trim()).filter((t) => t.length > 0)
  },
})

async function deletePolicy(name: string) {
  if (!confirm(`确定删除降采样策略 ${name}？`)) return
  try {
    await apiDelete(`/api/v1/admin/downsample/policies/${encodeURIComponent(name)}`)
    await loadData()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
  }
}

async function togglePolicy(policy: DownsamplePolicy) {
  const action = policy.enabled ? 'pause' : 'resume'
  try {
    await apiPost(`/api/v1/admin/downsample/policies/${encodeURIComponent(policy.name)}/${action}`)
    await loadData()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '操作失败'
  }
}

function addPolicyFunction() {
  newPolicy.value.functions.push({ function: 'mean', field: '', as: '' })
}

function removePolicyFunction(idx: number) {
  if (newPolicy.value.functions.length > 1) newPolicy.value.functions.splice(idx, 1)
}

function formatDuration(ns: number): string {
  if (ns >= 3600e9) return (ns / 3600e9).toFixed(1) + 'h'
  if (ns >= 60e9) return (ns / 60e9).toFixed(0) + 'm'
  if (ns >= 1e9) return (ns / 1e9).toFixed(0) + 's'
  return ns + 'ns'
}

function formatUnix(ts: number): string {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString()
}

function getStatus(name: string): DownsampleStatus | undefined {
  return statuses.value.find((s) => s.policy_name === name)
}
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <p v-if="loadError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ loadError }}</p>
    <p v-if="actionError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>
    <div class="flex gap-2">
      <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="showCreate = true"><Plus class="h-4 w-4" /> 创建策略</button>
      <button class="inline-flex items-center gap-1 rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-600 hover:bg-slate-50" @click="loadData"><RefreshCw class="h-4 w-4" /> 刷新</button>
    </div>
    <div class="rounded-xl border border-slate-200 bg-white">
      <div v-if="!policies.length" class="p-6 text-center text-sm text-slate-400">暂无降采样策略</div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 text-left">
            <th class="px-4 py-3 text-xs font-medium text-slate-500">名称</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">源 → 目标</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">间隔</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">状态</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">进度</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="policy in policies" :key="policy.name" class="border-b border-slate-100 last:border-b-0">
            <td class="px-4 py-3 font-medium text-slate-700">{{ policy.name }}</td>
            <td class="px-4 py-3 text-slate-600">{{ policy.source_database }}/{{ policy.source_measurement }} → {{ policy.target_database }}/{{ policy.target_measurement }}</td>
            <td class="px-4 py-3 text-slate-600">{{ formatDuration(policy.interval) }}</td>
            <td class="px-4 py-3"><span :class="policy.enabled ? 'bg-green-100 text-green-700' : 'bg-slate-100 text-slate-500'" class="rounded px-2 py-0.5 text-xs font-medium">{{ policy.enabled ? '运行中' : '已暂停' }}</span></td>
            <td class="px-4 py-3 text-xs text-slate-500">{{ getStatus(policy.name) ? formatUnix(getStatus(policy.name)!.completed_until_unix) : '-' }}</td>
            <td class="px-4 py-3">
              <div class="flex items-center gap-1">
                <button class="rounded p-1 text-slate-400 hover:text-slate-600" :title="policy.enabled ? '暂停' : '恢复'" @click="togglePolicy(policy)"><component :is="policy.enabled ? Pause : Play" class="h-4 w-4" /></button>
                <button class="rounded p-1 text-slate-400 hover:text-red-600" @click="deletePolicy(policy.name)"><Trash2 class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showCreate = false">
      <div class="w-[480px] max-h-[80vh] overflow-auto rounded-xl bg-white p-6 shadow-lg">
        <h3 class="mb-4 text-sm font-semibold text-slate-800">创建降采样策略</h3>
        <div class="grid grid-cols-2 gap-3">
          <div><label class="mb-1 block text-xs text-slate-500">名称</label><input v-model="newPolicy.name" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500">源数据库</label><input v-model="newPolicy.source_database" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500">源 Measurement</label><input v-model="newPolicy.source_measurement" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500">目标数据库</label><input v-model="newPolicy.target_database" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500">目标 Measurement</label><input v-model="newPolicy.target_measurement" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500">间隔 (纳秒)</label><input v-model.number="newPolicy.interval" type="number" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
        </div>
        <div class="mt-3">
          <label class="mb-1 block text-xs text-slate-500">Functions</label>
          <div class="space-y-1.5">
            <div v-for="(fn, idx) in newPolicy.functions" :key="idx" class="flex items-center gap-1.5">
              <select v-model="fn.function" class="rounded border border-slate-300 px-1.5 py-1 text-xs">
                <option value="mean">mean</option>
                <option value="sum">sum</option>
                <option value="min">min</option>
                <option value="max">max</option>
                <option value="first">first</option>
                <option value="last">last</option>
                <option value="count">count</option>
              </select>
              <input v-model="fn.field" placeholder="field" class="flex-1 rounded border border-slate-300 px-1.5 py-1 text-xs" />
              <input v-model="fn.as" placeholder="as" class="flex-1 rounded border border-slate-300 px-1.5 py-1 text-xs" />
              <button class="rounded p-0.5 text-slate-400 hover:text-red-600" @click="removePolicyFunction(idx)"><Trash2 class="h-3.5 w-3.5" /></button>
            </div>
          </div>
          <button class="mt-1.5 inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-700" @click="addPolicyFunction"><Plus class="h-3 w-3" /> 添加 function</button>
        </div>
        <div class="mt-3">
          <label class="mb-1 block text-xs text-slate-500">Group By Tags (逗号分隔)</label>
          <input v-model="newPolicyTagsText" placeholder="host, region" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" />
        </div>
        <div class="mt-4 flex justify-end gap-2">
          <button class="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100" @click="showCreate = false">取消</button>
          <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="createPolicy">创建</button>
        </div>
      </div>
    </div>
  </div>
</template>
