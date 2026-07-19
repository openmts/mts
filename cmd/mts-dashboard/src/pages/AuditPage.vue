<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import { useI18n } from '@/composables/useI18n'
import { useNotify } from '@/composables/useNotify'
import { ScrollText, Download, RefreshCw } from 'lucide-vue-next'

interface User { name: string; display_name?: string }
interface UsersResponse { users: User[] }
interface AuditEvent {
  time: string
  user_name: string
  action: string
  database?: string
  detail?: string
}
interface AuditResponse { events: AuditEvent[]; total?: number }

const { isAdmin } = useAuth()
const { t } = useI18n()
const { success, error: notifyError } = useNotify()
const users = ref<User[]>([])
const selectedUser = ref('')
const actionFilter = ref('')
const sinceLocal = ref('')
const untilLocal = ref('')
const auditEvents = ref<AuditEvent[]>([])
const loading = ref(false)
const loadError = ref('')

onMounted(async () => {
  if (!isAdmin.value) return
  try {
    const data = await apiGet<UsersResponse>('/api/v1/users')
    users.value = data.users ?? []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载用户失败'
  }
  await loadAudit()
})

function toUnix(local: string): number | undefined {
  if (!local) return undefined
  const ms = Date.parse(local)
  if (Number.isNaN(ms)) return undefined
  return Math.floor(ms / 1000)
}

async function loadAudit() {
  loading.value = true
  loadError.value = ''
  try {
    const params = new URLSearchParams()
    if (selectedUser.value) params.set('user_name', selectedUser.value)
    if (actionFilter.value.trim()) params.set('action', actionFilter.value.trim())
    const since = toUnix(sinceLocal.value)
    const until = toUnix(untilLocal.value)
    if (since) params.set('since_unix', String(since))
    if (until) params.set('until_unix', String(until))
    params.set('limit', '500')
    const qs = params.toString()
    const data = await apiGet<AuditResponse>(`/api/v1/admin/audit${qs ? `?${qs}` : ''}`)
    auditEvents.value = data.events ?? []
  } catch (e) {
    // fallback: per-user endpoint if global unavailable
    if (selectedUser.value) {
      try {
        const data = await apiGet<AuditResponse>(`/api/v1/users/${encodeURIComponent(selectedUser.value)}/audit`)
        auditEvents.value = data.events ?? []
        loadError.value = ''
        return
      } catch (e2) {
        loadError.value = e2 instanceof Error ? e2.message : '加载审计日志失败'
      }
    } else {
      loadError.value = e instanceof Error ? e.message : '加载审计日志失败'
    }
    auditEvents.value = []
    notifyError(loadError.value)
  } finally {
    loading.value = false
  }
}

function exportJSON() {
  const blob = new Blob([JSON.stringify(auditEvents.value, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `mts-audit-${Date.now()}.json`
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  success('审计日志已导出')
}

const filteredCount = computed(() => auditEvents.value.length)
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <p v-if="loadError" class="rounded-lg border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/40 p-3 text-sm text-red-700 dark:text-red-200">{{ loadError }}</p>

    <div class="grid gap-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900 md:grid-cols-5">
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('user') }}
        <select v-model="selectedUser" class="mt-1 w-full rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm dark:border-slate-600 dark:bg-slate-800">
          <option value="">全部用户</option>
          <option v-for="user in users" :key="user.name" :value="user.name">{{ user.display_name || user.name }}</option>
        </select>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('action') }}
        <input v-model="actionFilter" placeholder="login / flush ..." class="mt-1 w-full rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm dark:border-slate-600 dark:bg-slate-800" />
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('since') }}
        <input v-model="sinceLocal" type="datetime-local" class="mt-1 w-full rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm dark:border-slate-600 dark:bg-slate-800" />
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('until') }}
        <input v-model="untilLocal" type="datetime-local" class="mt-1 w-full rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm dark:border-slate-600 dark:bg-slate-800" />
      </label>
      <div class="flex items-end gap-2">
        <button :disabled="loading" class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-slate-100 dark:bg-slate-800 dark:text-slate-900" @click="loadAudit">
          <span class="inline-flex items-center gap-1"><RefreshCw class="h-3.5 w-3.5" />{{ loading ? t('loading') : t('filter') }}</span>
        </button>
        <button class="rounded-lg border border-slate-200 px-3 py-2 text-sm dark:border-slate-700" :disabled="!auditEvents.length" @click="exportJSON">
          <span class="inline-flex items-center gap-1"><Download class="h-3.5 w-3.5" />{{ t('export') }}</span>
        </button>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
      <div class="flex items-center justify-between border-b border-slate-100 px-4 py-2 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500 dark:border-slate-800">
        <span class="inline-flex items-center gap-1"><ScrollText class="h-3.5 w-3.5" /> 审计事件</span>
        <span>{{ filteredCount }} 条</span>
      </div>
      <div v-if="!auditEvents.length" class="p-6 text-center text-sm text-slate-400 dark:text-slate-500">
        <template v-if="loading">{{ t('loading') }}</template>
        <template v-else>暂无审计记录</template>
      </div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 dark:border-slate-700 text-left dark:border-slate-800">
            <th class="px-4 py-3 text-xs font-medium text-slate-500 dark:text-slate-400 dark:text-slate-500">时间</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500 dark:text-slate-400 dark:text-slate-500">用户</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500 dark:text-slate-400 dark:text-slate-500">操作</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500 dark:text-slate-400 dark:text-slate-500">库</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500 dark:text-slate-400 dark:text-slate-500">详情</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(evt, idx) in auditEvents" :key="idx" class="border-b border-slate-100 last:border-b-0 dark:border-slate-800">
            <td class="px-4 py-3 text-xs text-slate-600 dark:text-slate-300">{{ evt.time }}</td>
            <td class="px-4 py-3 text-xs text-slate-700 dark:text-slate-200">{{ evt.user_name }}</td>
            <td class="px-4 py-3 text-xs font-medium text-slate-700 dark:text-slate-200">{{ evt.action }}</td>
            <td class="px-4 py-3 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ evt.database || '—' }}</td>
            <td class="px-4 py-3 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ evt.detail || '—' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
