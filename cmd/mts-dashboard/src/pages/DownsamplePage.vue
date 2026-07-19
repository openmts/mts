<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { apiGet, apiPost, apiDelete } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import { useNotify } from '@/composables/useNotify'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
import { parseHumanDurationToNs, formatNsDuration } from '@/utils/duration'
import { Plus, Trash2, Play, Pause, RefreshCw, PlayCircle, RotateCcw, FlaskConical } from 'lucide-vue-next'
import type { DownsamplePolicy, DownsampleStatus, DownsampleRunResult, DownsampleDryRunResult } from '@/api/types'

interface PoliciesResponse { policies: DownsamplePolicy[] }
interface StatusesResponse { statuses: DownsampleStatus[] }

const { isAdmin } = useAuth()
const { success, error: notifyError } = useNotify()
const policies = ref<DownsamplePolicy[]>([])
const statuses = ref<DownsampleStatus[]>([])
const loadError = ref('')
const actionResult = ref<ActionResult | null>(null)
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
  actionResult.value = null
  try {
    newPolicy.value.interval = parseHumanDurationToNs(intervalHuman.value)
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'interval 无效'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  if (!newPolicy.value.source_database || !newPolicy.value.source_measurement) {
    const msg = '请填写源库与 measurement'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
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
    actionResult.value = makeActionResult('ok', '降采样策略已创建')
    success('降采样策略已创建')
  } catch (e) {
    const msg = e instanceof Error ? e.message : '创建失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
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
    actionResult.value = makeActionResult('ok', '策略已删除')
    success('策略已删除')
  } catch (e) {
    const msg = e instanceof Error ? e.message : '删除失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  } finally {
    deleteLoading.value = false
  }
}

async function togglePolicy(policy: DownsamplePolicy) {
  const action = policy.enabled ? 'disable' : 'enable'
  try {
    await apiPost(`/api/v1/admin/downsample/policies/${encodeURIComponent(policy.name)}/${action}`)
    await loadData()
    const msg = policy.enabled ? '策略已禁用' : '策略已启用'
    actionResult.value = makeActionResult('ok', msg)
    success(msg)
  } catch (e) {
    const msg = e instanceof Error ? e.message : '操作失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
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

async function runPolicy(name: string) {
  try {
    const data = await apiPost<{ result: DownsampleRunResult }>(`/api/v1/admin/downsample/policies/${encodeURIComponent(name)}/run`, {})
    const r = data.result
    const msg = `run ${name}: windows=${r?.windows_processed ?? 0} points=${r?.points_written ?? 0}`
    actionResult.value = makeActionResult('ok', msg)
    await loadData()
    success(msg)
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'run 失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}

async function resetPolicy(name: string) {
  try {
    await apiPost(`/api/v1/admin/downsample/policies/${encodeURIComponent(name)}/reset`, {
      reset: { allow_policy_replace: true },
    })
    await loadData()
    const msg = `策略 ${name} 已重置`
    actionResult.value = makeActionResult('ok', msg)
    success(msg)
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'reset 失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}

async function dryRunPolicy(name: string) {
  try {
    const data = await apiPost<{ result: DownsampleDryRunResult }>(
      `/api/v1/admin/downsample/policies/${encodeURIComponent(name)}/dry-run`,
      {},
    )
    const r = data.result
    const msg = `dry-run ${name}: windows=${r?.windows ?? 0} points≈${r?.points_estimate ?? 0} samples≈${r?.samples_estimate ?? 0} complete=${r?.estimate_complete}`
    actionResult.value = makeActionResult('info', msg)
    success(msg)
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'dry-run 失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
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
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">降采样</h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">策略与执行状态</p>
      </div>
      <div class="flex gap-2">
        <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900 px-3 py-1.5 text-xs" @click="loadData"><RefreshCw class="h-3.5 w-3.5" /> 刷新</button>
        <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white" @click="showCreate = true"><Plus class="h-3.5 w-3.5" /> 创建策略</button>
      </div>
    </div>
    <ActionResultBanner
      v-if="loadError"
      kind="error"
      :message="loadError"
      @dismiss="loadError = ''"
    />
    <ActionResultBanner
      :result="actionResult"
      @dismiss="actionResult = null"
    />

    <div v-if="!policies.length" class="mts-card">
      <EmptyState
        title="暂无降采样策略"
        description="创建策略后可在此执行 run / dry-run / reset，并查看各策略状态。"
      >
        <template #action>
          <button type="button" class="mts-btn-primary" @click="showCreate = true">创建策略</button>
        </template>
      </EmptyState>
    </div>

    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-100 bg-slate-50/50 text-left text-xs uppercase text-slate-500 dark:text-slate-400 dark:text-slate-500">
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
            <td class="px-4 py-3 font-medium text-slate-700 dark:text-slate-200">{{ policy.name }}</td>
            <td class="px-4 py-3 text-slate-600 dark:text-slate-300">{{ policy.source_database }}/{{ policy.source_measurement }} → {{ policy.target_database }}/{{ policy.target_measurement }}</td>
            <td class="px-4 py-3 text-slate-600 dark:text-slate-300">{{ formatDuration(policy.interval) }}</td>
            <td class="px-4 py-3"><span :class="policy.enabled ? 'bg-green-100 text-green-700 dark:text-green-200' : 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 dark:text-slate-500'" class="rounded px-2 py-0.5 text-xs font-medium">{{ policy.enabled ? '已启用' : '已禁用' }}</span></td>
            <td class="px-4 py-3 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ getStatus(policy.name) ? formatUnix(getStatus(policy.name)!.completed_until_unix) : '-' }}</td>
            <td class="px-4 py-3">
              <div class="flex items-center gap-1">
                                <button class="rounded p-1 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200" title="run" @click="runPolicy(policy.name)"><PlayCircle class="h-4 w-4" /></button>
                <button class="rounded p-1 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200" title="dry-run" @click="dryRunPolicy(policy.name)"><FlaskConical class="h-4 w-4" /></button>
                <button class="rounded p-1 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200" title="reset" @click="resetPolicy(policy.name)"><RotateCcw class="h-4 w-4" /></button>
                <button class="rounded p-1 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200" @click="togglePolicy(policy)"><component :is="policy.enabled ? Pause : Play" class="h-4 w-4" /></button>
                <button class="rounded p-1 text-slate-400 dark:text-slate-500 hover:text-red-600 dark:text-red-300" @click="requestDelete(policy.name)"><Trash2 class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4" @click.self="showCreate = false" @keydown.esc="showCreate = false">
      <div class="w-[480px] max-h-[80vh] overflow-auto rounded-xl bg-white p-6 shadow-lg" role="dialog" aria-modal="true">
        <h3 class="mb-4 text-sm font-semibold text-slate-800 dark:text-slate-100">创建降采样策略</h3>
        <div class="grid grid-cols-2 gap-3">
          <div><label class="mb-1 block text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">名称</label><input v-model="newPolicy.name" class="w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-xs" /></div>
          <div>
            <label class="mb-1 block text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">间隔</label>
            <input v-model="intervalHuman" placeholder="1m / 5m / 1h / 1d" class="w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-xs" />
            <p class="mt-1 text-[11px] text-slate-400 dark:text-slate-500">支持 ns/us/ms/s/m/h/d，例如 30s、1m、1h</p>
          </div>
          <div><label class="mb-1 block text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">源数据库</label><input v-model="newPolicy.source_database" class="w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">源 Measurement</label><input v-model="newPolicy.source_measurement" class="w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">目标数据库</label><input v-model="newPolicy.target_database" class="w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-xs" /></div>
          <div><label class="mb-1 block text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">目标 Measurement</label><input v-model="newPolicy.target_measurement" class="w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-xs" /></div>
        </div>
        <div class="mt-3">
          <label class="mb-1 block text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">Functions</label>
          <div class="space-y-1.5">
            <div v-for="(fn, idx) in newPolicy.functions" :key="idx" class="flex items-center gap-1.5">
              <select v-model="fn.function" class="rounded border border-slate-300 dark:border-slate-600 px-1.5 py-1 text-xs">
                <option v-for="opt in ['mean','sum','min','max','first','last','count']" :key="opt" :value="opt">{{ opt }}</option>
              </select>
              <input v-model="fn.field" placeholder="field" class="flex-1 rounded border border-slate-300 dark:border-slate-600 px-1.5 py-1 text-xs" />
              <input v-model="fn.as" placeholder="as" class="flex-1 rounded border border-slate-300 dark:border-slate-600 px-1.5 py-1 text-xs" />
              <button class="rounded p-0.5 text-slate-400 dark:text-slate-500 hover:text-red-600 dark:text-red-300" @click="removePolicyFunction(idx)"><Trash2 class="h-3.5 w-3.5" /></button>
            </div>
          </div>
          <button class="mt-1.5 inline-flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500" @click="addPolicyFunction"><Plus class="h-3 w-3" /> 添加 function</button>
        </div>
        <div class="mt-3">
          <label class="mb-1 block text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">Group By Tags (逗号分隔)</label>
          <input v-model="newPolicyTagsText" placeholder="host, region" class="w-full rounded border border-slate-300 dark:border-slate-600 px-2 py-1.5 text-xs" />
        </div>
        <div class="mt-4 flex justify-end gap-2">
          <button class="rounded-lg px-4 py-2 text-sm text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:bg-slate-800" @click="showCreate = false">取消</button>
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
