<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost, apiPut, apiDelete } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { Plus, Trash2, Key, Lock } from 'lucide-vue-next'
import UserModals from '@/components/UserModals.vue'
import UserGrantPanel from '@/components/UserGrantPanel.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useNotify } from '@/composables/useNotify'

interface User { name: string; display_name?: string; role?: string; disabled?: boolean; metadata?: Record<string, string> }
interface UsersResponse { users: User[] }
interface DatabaseGrant { database: string; permission: string }
interface PermissionsResponse { grants: DatabaseGrant[] }

const users = ref<User[]>([])
const databases = ref<string[]>([])
const { currentUser, isAdmin } = useAuth()
const { success, error: notifyError } = useNotify()
const loadError = ref('')
const actionError = ref('')
const showCreate = ref(false)
const newUser = ref({ name: '', display_name: '', password: '', role: 'user' })
const selectedUser = ref<User | null>(null)
const showSetPassword = ref(false)
const setPasswordUser = ref('')
const setPasswordValue = ref('')
const showChangeSelfPassword = ref(false)
const selfOldPassword = ref('')
const selfNewPassword = ref('')
const userGrants = ref<DatabaseGrant[]>([])
const grantDbs = ref<string[]>([])
const grantPerms = ref<string[]>([])
const deleteOpen = ref(false)
const deleteName = ref('')
const deleteLoading = ref(false)

onMounted(async () => {
  if (!isAdmin.value) return
  await loadUsers()
  try {
    const { listDatabases } = await import('@/api/meta')
    databases.value = await listDatabases()
  } catch (_) { /* 非关键 */ }
})

async function loadUsers() {
  try {
    const data = await apiGet<UsersResponse>('/api/v1/users')
    users.value = data.users ?? []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
}

async function createUser() {
  if (!newUser.value.name.trim()) return
  actionError.value = ''
  try {
    await apiPost('/api/v1/users', {
      name: newUser.value.name.trim(),
      display_name: newUser.value.display_name,
      role: newUser.value.role,
      password: newUser.value.password || undefined,
    })
    showCreate.value = false
    newUser.value = { name: '', display_name: '', role: 'user', password: '' }
    await loadUsers()
    success('用户已创建')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建失败'
    notifyError(actionError.value)
  }
}

async function doSetPassword() {
  if (!setPasswordValue.value) return
  actionError.value = ''
  try {
    await apiPut(`/api/v1/users/${encodeURIComponent(setPasswordUser.value)}/password`, { password: setPasswordValue.value })
    showSetPassword.value = false
    setPasswordValue.value = ''
    success('密码已设置')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '设置密码失败'
    notifyError(actionError.value)
  }
}

async function doChangeSelfPassword() {
  if (!selfOldPassword.value || !selfNewPassword.value) return
  actionError.value = ''
  try {
    await apiPost('/api/v1/auth/password', {
      user_name: currentUser.value,
      old_password: selfOldPassword.value,
      new_password: selfNewPassword.value,
    })
    showChangeSelfPassword.value = false
    selfOldPassword.value = ''
    selfNewPassword.value = ''
    success('密码已修改')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '修改密码失败'
    notifyError(actionError.value)
  }
}

function requestDelete(name: string) {
  deleteName.value = name
  deleteOpen.value = true
}

async function confirmDelete() {
  const name = deleteName.value
  if (!name) return
  deleteLoading.value = true
  try {
    await apiDelete(`/api/v1/users/${encodeURIComponent(name)}`)
    await loadUsers()
    if (selectedUser.value?.name === name) selectedUser.value = null
    deleteOpen.value = false
    success(`用户 ${name} 已删除`)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
    notifyError(actionError.value)
  } finally {
    deleteLoading.value = false
  }
}

async function toggleDisable(user: User) {
  try {
    await apiPut(`/api/v1/users/${encodeURIComponent(user.name)}`, { ...user, disabled: !user.disabled })
    await loadUsers()
    success(user.disabled ? '用户已启用' : '用户已禁用')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '操作失败'
    notifyError(actionError.value)
  }
}

async function selectUser(user: User) {
  selectedUser.value = user
  try {
    const data = await apiGet<PermissionsResponse>(`/api/v1/users/${encodeURIComponent(user.name)}/database-permissions`)
    userGrants.value = data.grants ?? []
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '加载权限失败'
  }
}

function toggleGrantDb(db: string) {
  const idx = grantDbs.value.indexOf(db)
  if (idx >= 0) grantDbs.value.splice(idx, 1)
  else grantDbs.value.push(db)
}

function toggleGrantPerm(perm: string) {
  const idx = grantPerms.value.indexOf(perm)
  if (idx >= 0) grantPerms.value.splice(idx, 1)
  else grantPerms.value.push(perm)
}

async function grantPermission() {
  if (!grantDbs.value.length || !grantPerms.value.length || !selectedUser.value) return
  actionError.value = ''
  try {
    const tasks: Promise<unknown>[] = []
    for (const db of grantDbs.value) {
      for (const perm of grantPerms.value) {
        tasks.push(apiPut(`/api/v1/users/${encodeURIComponent(selectedUser.value.name)}/database-permissions/${encodeURIComponent(db)}/${perm}`))
      }
    }
    await Promise.all(tasks)
    grantDbs.value = []
    grantPerms.value = []
    await selectUser(selectedUser.value)
    success('权限已授予')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '授权失败'
    notifyError(actionError.value)
  }
}

async function revokeGrant(g: DatabaseGrant) {
  if (!selectedUser.value) return
  try {
    await apiDelete(`/api/v1/users/${encodeURIComponent(selectedUser.value.name)}/database-permissions/${encodeURIComponent(g.database)}/${encodeURIComponent(g.permission)}`)
    await selectUser(selectedUser.value)
    success('权限已撤销')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '撤销失败'
    notifyError(actionError.value)
  }
}

function openSetPassword(name: string) {
  setPasswordUser.value = name
  setPasswordValue.value = ''
  showSetPassword.value = true
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">用户</h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">账号、角色与库权限</p>
      </div>
      <div class="flex gap-2">
        <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900 px-3 py-1.5 text-xs text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800" @click="showChangeSelfPassword = true">
          <Lock class="h-3.5 w-3.5" /> 修改我的密码
        </button>
        <button v-if="isAdmin" class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700" @click="showCreate = true">
          <Plus class="h-3.5 w-3.5" /> 新建用户
        </button>
      </div>
    </div>

    <p v-if="loadError" class="rounded-xl border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/40 p-4 text-sm text-red-700 dark:text-red-200">{{ loadError }}</p>
    <p v-if="actionError" class="rounded-xl border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/40 p-4 text-sm text-red-700 dark:text-red-200">{{ actionError }}</p>

    <div v-if="!isAdmin" class="rounded-xl border border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-900/60 p-4 text-sm text-slate-600 dark:text-slate-300">
      普通用户仅可修改自己的密码。用户列表与授权管理需管理员权限。
    </div>

    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-100 bg-slate-50/50 text-left text-xs uppercase text-slate-500 dark:text-slate-400 dark:text-slate-500">
            <th class="px-4 py-2.5">用户</th>
            <th class="px-4 py-2.5">角色</th>
            <th class="px-4 py-2.5">状态</th>
            <th class="px-4 py-2.5">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.name" class="border-b border-slate-50 hover:bg-slate-50 dark:hover:bg-slate-800/40">
            <td class="px-4 py-3">
              <button class="text-left font-medium text-slate-800 dark:text-slate-100 hover:underline" @click="selectUser(u)">{{ u.name }}</button>
              <span v-if="u.display_name" class="ml-2 text-xs text-slate-400 dark:text-slate-500">{{ u.display_name }}</span>
            </td>
            <td class="px-4 py-3 text-slate-600 dark:text-slate-300">{{ u.role || 'user' }}</td>
            <td class="px-4 py-3">
              <span :class="u.disabled ? 'bg-red-50 text-red-700 dark:text-red-200' : 'bg-green-50 text-green-700 dark:text-green-200'" class="rounded px-2 py-0.5 text-xs">{{ u.disabled ? '禁用' : '正常' }}</span>
            </td>
            <td class="px-4 py-3">
              <div class="flex items-center gap-1">
                <button class="rounded p-1 text-slate-400 dark:text-slate-500 hover:text-slate-700 dark:text-slate-200" title="设置密码" @click="openSetPassword(u.name)"><Key class="h-4 w-4" /></button>
                <button class="rounded px-2 py-0.5 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500 hover:bg-slate-100 dark:bg-slate-800" @click="toggleDisable(u)">{{ u.disabled ? '启用' : '禁用' }}</button>
                <button class="rounded p-1 text-slate-400 dark:text-slate-500 hover:text-red-600 dark:text-red-300" @click="requestDelete(u.name)"><Trash2 class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <UserGrantPanel
      v-if="isAdmin && selectedUser"
      :selected-user="selectedUser"
      :user-grants="userGrants"
      :databases="databases"
      :grant-dbs="grantDbs"
      :grant-perms="grantPerms"
      @close="selectedUser = null"
      @toggle-db="toggleGrantDb"
      @toggle-perm="toggleGrantPerm"
      @grant="grantPermission"
      @revoke="revokeGrant"
    />

    <UserModals
      v-model:show-create="showCreate"
      v-model:new-user="newUser"
      v-model:show-set-password="showSetPassword"
      v-model:set-password-user="setPasswordUser"
      v-model:set-password-value="setPasswordValue"
      v-model:show-change-self-password="showChangeSelfPassword"
      v-model:self-old-password="selfOldPassword"
      v-model:self-new-password="selfNewPassword"
      @create-user="createUser"
      @set-password="doSetPassword"
      @change-password="doChangeSelfPassword"
    />

    <ConfirmDialog
      v-model:open="deleteOpen"
      title="删除用户"
      :message="`确定删除用户 ${deleteName}？此操作不可恢复。`"
      confirm-label="删除"
      danger
      :loading="deleteLoading"
      @confirm="confirmDelete"
    />
  </div>
</template>
