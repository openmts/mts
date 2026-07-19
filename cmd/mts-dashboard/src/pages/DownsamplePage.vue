<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { apiGet, apiPost, apiDelete } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useNotify } from '@/composables/useNotify'
import { parseHumanDurationToNs, formatNsDuration } from '@/utils/duration'
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
const { success, error: notifyError } = useNotify()
const policies = ref<DownsamplePolicy[]>([])
const statuses = ref<DownsampleStatus[]>([])
const loadError = ref('')
const actionError = ref('')
const showCreate = ref(false)
const intervalHuman = ref('1m')
const deleteOpen = ref(false)
const deleteName = ref('')
const deleteLoading = ref(false)
const newPolicy = ref<DownsamplePolicy>({
  name: '', source_database: '', source_measurement: '',
  target_database: '', target_measurement: '',
  interval: 60_000_000_000, functions: [{ function: 'mean', field: 'value', as: 'mean_value' }],
  group_by_tags: [], enabled: true,
})

onMounted(loadData)

async function loadData() {
  if (!isAdmin.value) return
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

const newPolicyTagsText = computed({
  get: () => newPolicy.value.group_by_tags.join(', '),
  set: (v: string) => {
    newPolicy.value.group_by_tags = v.split(',').map((t) => t.trim()).filter((t) => t.length > 0)
  },
})

async function createPolicy() {
  if (!newPolicy.value.name.trim()) return
  actionError.value = ''
  try {
    newPolicy.value.interval = parseHumanDurationToNs(intervalHuman.value)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : 'interval 无效'
    notifyError(actionError.value)
    return
  }
  if (!newPolicy.value.source_database || !newPolicy.value.source_measurement) {
    actionError.value = '请填写源库与 measurement'
    notifyError(actionError.value)
    return
  }
  try {
    await apiPost('/api/v1/admin/downsample/policies', newPolicy.value)
    showCreate.value = false
    newPolicy.value = {
      name: '', source_database: '', source_measurement: '',
      target_database: '', target_measurement: '',
      interval: 60_000_000_000, functions: [{ function: 'mean', field: 'value', as: 'mean_value' }],
      group_by_tags: [], enabled: true,
    }
    intervalHuman.value = '1m'
    await loadData()
    success('降采样策略已创建')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建失败'
    notifyError(actionError.value)
  }
}

function requestDelete(name: string) {
  deleteName.value = name
  deleteOpen.value = true
}

async function confirmDelete() {
  deleteLoading.value = true
  try {
    await apiDelete(`/api/v1/admin/downsample/policies/${encodeURIComponent(deleteName.value)}`)
    deleteOpen.value = false
    await loadData()
    success('策略已删除')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
    notifyError(actionError.value)
  } finally {
    deleteLoading.value = false
  }
}

async function togglePolicy(policy: DownsamplePolicy) {
  const action = policy.enabled ? 'pause' : 'resume'
  try {
    await apiPost(`/api/v1/admin/downsample/policies/${encodeURIComponent(policy.name)}/${action}`)
    await loadData()
    success(policy.enabled ? '策略已暂停' : '策略已恢复')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '操作失败'
    notifyError(actionError.value)
  }
}

function addPolicyFunction() {
  newPolicy.value.functions.push({ function: 'mean', field: '', as: '' })
}
function removePolicyFunction(idx: number) {
  newPolicy.value.functions.splice(idx, 1)
}
function getStatus(name: string) {
  return statuses.value.find((s) => s.policy_name === name)
}
function formatUnix(v: number) {
  if (!v) return '-'
  return new Date(v > 1e12 ? v / 1e6 : v * 1000).toLocaleString()
}
function formatDuration(ns: number) {
  try { return formatNsDuration(ns) } catch { return String(ns) }
}
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800">降采样</h1>
        <p class="text-xs text-slate-500">策略与执行状态</p>
      </div>
      <div class="flex gap-2">
        <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs" @click="loadData"><RefreshCw class="h-3.5 w-3.5" /> 刷新</button>
        <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white" @click="showCreate = true"><Plus class="h-3.5 w-3.5" /> 创建策略</button>
      </div>
    </div>
    <p v-if="loadError" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ loadError }}</p>
    <p v-if="actionError" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ actionError }}</p>

    <div class="overflow-hidden rounded-xl border border-slate-200 bg-white">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-100 bg-slate-50/50 text-left text-xs uppercase text-slate-500">
            <th class="px-4 py-2.5">名称</th>
            <th class="px-4 py-2.5">路径</th>
            <th class="px-4 py-2.5">间隔</th>
            <th class="px-4 py-2.5">状态</th>
            <th class="px-4 py-2.5">完成至</th>
            <th class="px-4 py-2.5">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="policy in policies" :key="policy.name" class="border-b border-slate-50">
            <td class="px-4 py-3 font-medium text-slate-700">{{ policy.name }}</td>
            <td class="px-4 py-3 text-slate-600">{{ policy.source_database }}/{{ policy.source_measurement }} → {{ policy.target_database }}/{{ policy.target_measurement }}</td>
            <td class="px-4 py-3 text-slate-600">{{ formatDuration(policy.interval) }}</td>
            <td class="px-4 py-3"><span :class="policy.enabled ? 'bg-green-100 text-green-700' : 'bg-slate-100 text-slate-500'" class="rounded px-2 py-0.5 text-xs font-medium">{{ policy.enabled ? '运行中' : '已暂停' }}</span></td>
            <td class="px-4 py-3 text-xs text-slate-500">{{ getStatus(policy.name) ? formatUnix(getStatus(policy.name)!.completed_until_unix) : '-' }}</td>
            <td class="px-4 py-3">
              <div class="flex items-center gap-1">
                <button class="rounded p-1 text-slate-400 hover:text-slate-600" @click="togglePolicy(policy)"><component :is="policy.enabled ? Pause : Play" class="h-4 w-4" /></button>
                <button class="rounded p-1 text-slate-400 hover:text-red-600" @click="requestDelete(policy.name)"><Trash2 class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4" @click.self="showCreate = false" @keydown.esc="showCreate = false">
      <div class="w-[480px] max-h-[80vh] overflow-auto rounded-xl bg-white p-6 shadow-lg" role="dialog" aria-modal="true">
        <h3 class="mb-4 text-sm font-semibold text-slate-800">创建降采样策略</h3>
        <div class="grid grid-cols-2 gap-3">
          <div><label class="mb-1 block text-xs text-slate-500">名称</label><input v-model="newPolicy.name" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
          <div>
            <label class="mb-1 block text-xs text-slate-500">间隔</label>
            <input v-model="intervalHuman" placeholder="1m / 5m / 1h / 1d" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" />
            <p class="mt-1 text-[11px] text-slate-400">支持 ns/us/ms/s/m/h/d，例如 30s、1m、1h</p>
          </div>
          <div><label class="mb-1 block text-xs text-slate-500">源数据库</label><input v-model="newPolicy.source_database" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500">源 Measurement</label><input v-model="newPolicy.source_measurement" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500">目标数据库</label><input v-model="newPolicy.target_database" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500">目标 Measurement</label><input v-model="newPolicy.target_measurement" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" /></div>
        </div>
        <div class="mt-3">
          <label class="mb-1 block text-xs text-slate-500">Functions</label>
          <div class="space-y-1.5">
            <div v-for="(fn, idx) in newPolicy.functions" :key="idx" class="flex items-center gap-1.5">
              <select v-model="fn.function" class="rounded border border-slate-300 px-1.5 py-1 text-xs">
                <option v-for="opt in ['mean','sum','min','max','first','last','count']" :key="opt" :value="opt">{{ opt }}</option>
              </select>
              <input v-model="fn.field" placeholder="field" class="flex-1 rounded border border-slate-300 px-1.5 py-1 text-xs" />
              <input v-model="fn.as" placeholder="as" class="flex-1 rounded border border-slate-300 px-1.5 py-1 text-xs" />
              <button class="rounded p-0.5 text-slate-400 hover:text-red-600" @click="removePolicyFunction(idx)"><Trash2 class="h-3.5 w-3.5" /></button>
            </div>
          </div>
          <button class="mt-1.5 inline-flex items-center gap-1 text-xs text-slate-500" @click="addPolicyFunction"><Plus class="h-3 w-3" /> 添加 function</button>
        </div>
        <div class="mt-3">
          <label class="mb-1 block text-xs text-slate-500">Group By Tags (逗号分隔)</label>
          <input v-model="newPolicyTagsText" placeholder="host, region" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" />
        </div>
        <div class="mt-4 flex justify-end gap-2">
          <button class="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100" @click="showCreate = false">取消</button>
          <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white" @click="createPolicy">创建</button>
        </div>
      </div>
    </div>

    <ConfirmDialog
      v-model:open="deleteOpen"
      title="删除降采样策略"
      :message="`确定删除策略 ${deleteName}？`"
      confirm-label="删除"
      danger
      :loading="deleteLoading"
      @confirm="confirmDelete"
    />
  </div>
</template>
