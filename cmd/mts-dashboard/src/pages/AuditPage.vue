<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import { ScrollText } from 'lucide-vue-next'

interface User { name: string; display_name?: string }
interface UsersResponse { users: User[] }
interface AuditEvent { time: string; user_name: string; action: string; detail: string }
interface AuditResponse { events: AuditEvent[] }

const { currentUser, isAdmin } = useAuth()
const users = ref<User[]>([])
const selectedUser = ref('')
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
  selectedUser.value = currentUser.value
  if (selectedUser.value) {
    await loadAudit()
  }
})

async function loadAudit() {
  if (!selectedUser.value) return
  loading.value = true
  loadError.value = ''
  try {
    const data = await apiGet<AuditResponse>(`/api/v1/users/${encodeURIComponent(selectedUser.value)}/audit`)
    auditEvents.value = data.events ?? []
  } catch (e) {
    auditEvents.value = []
    loadError.value = e instanceof Error ? e.message : '加载审计日志失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <p v-if="loadError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ loadError }}</p>

    <div class="flex items-end gap-3">
      <div>
        <label class="mb-1 block text-xs text-slate-500">选择用户</label>
        <select v-model="selectedUser" class="w-48 rounded-lg border border-slate-300 px-3 py-2 text-sm" @change="loadAudit">
          <option value="">-- 选择用户 --</option>
          <option v-for="user in users" :key="user.name" :value="user.name">{{ user.display_name || user.name }}</option>
        </select>
      </div>
      <button :disabled="!selectedUser || loading" class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50" @click="loadAudit">{{ loading ? '加载中...' : '查询' }}</button>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white">
      <div v-if="!auditEvents.length && selectedUser" class="p-6 text-center text-sm text-slate-400">
        <template v-if="loading">加载中...</template>
        <template v-else>
          <div class="flex flex-col items-center gap-2">
            <ScrollText class="h-8 w-8 text-slate-300" />
            <span>暂无审计记录</span>
          </div>
        </template>
      </div>
      <div v-else-if="!selectedUser" class="p-6 text-center text-sm text-slate-400">请选择用户查看审计日志</div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 text-left">
            <th class="px-4 py-3 text-xs font-medium text-slate-500">时间</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">操作</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">详情</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(evt, idx) in auditEvents" :key="idx" class="border-b border-slate-100 last:border-b-0">
            <td class="px-4 py-3 text-xs text-slate-600">{{ evt.time }}</td>
            <td class="px-4 py-3 text-xs font-medium text-slate-700">{{ evt.action }}</td>
            <td class="px-4 py-3 text-xs text-slate-500">{{ evt.detail }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
