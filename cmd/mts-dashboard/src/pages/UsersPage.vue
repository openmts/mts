<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { apiGet, apiPost, apiPut, apiDelete } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { Plus, Trash2, Shield, X, Key, Lock } from 'lucide-vue-next'
import UserModals from '@/components/UserModals.vue'
import { useNotify } from '@/composables/useNotify'

interface User { name: string; display_name?: string; role?: string; disabled?: boolean; metadata?: Record<string, string> }
interface UsersResponse { users: User[] }
interface DatabaseGrant { database: string; permission: string }
interface PermissionsResponse { grants: DatabaseGrant[] }
interface DatabaseListResponse { measurements: string[] }

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

onMounted(async () => {
  if (!isAdmin.value) return
  await loadUsers()
  try {
    const { listDatabases } = await import('@/api/meta')
    databases.value = await listDatabases()
  } catch (_) {
    // 非关键路径
  }
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
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建失败'
  }
}

async function doSetPassword() {
  if (!setPasswordValue.value) return
  actionError.value = ''
  try {
    await apiPut(`/api/v1/users/${encodeURIComponent(setPasswordUser.value)}/password`, { password: setPasswordValue.value })
    showSetPassword.value = false
    setPasswordValue.value = ''
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '设置密码失败'
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

async function deleteUser(name: string) {
  if (!confirm(`确定删除用户 ${name}？`)) return
  try {
    await apiDelete(`/api/v1/users/${encodeURIComponent(name)}`)
    await loadUsers()
    if (selectedUser.value?.name === name) selectedUser.value = null
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
  }
}

async function toggleDisable(user: User) {
  try {
    await apiPut(`/api/v1/users/${encodeURIComponent(user.name)}`, { ...user, disabled: !user.disabled })
    await loadUsers()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '操作失败'
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
  if (idx >= 0) {
    grantDbs.value.splice(idx, 1)
  } else {
    grantDbs.value.push(db)
  }
}

function toggleGrantPerm(perm: string) {
  const idx = grantPerms.value.indexOf(perm)
  if (idx >= 0) {
    grantPerms.value.splice(idx, 1)
  } else {
    grantPerms.value.push(perm)
  }
}

async function grantPermission() {
  if (!grantDbs.value.length || !grantPerms.value.length || !selectedUser.value) return
  actionError.value = ''
  const tasks: Promise<unknown>[] = []
  for (const db of grantDbs.value) {
    for (const perm of grantPerms.value) {
      tasks.push(apiPut(`/api/v1/users/${encodeURIComponent(selectedUser.value.name)}/database-permissions/${encodeURIComponent(db)}/${perm}`))
    }
  }
  const results = await Promise.allSettled(tasks)
  const failed = results.filter((r) => r.status === 'rejected')
  if (failed.length > 0) {
    actionError.value = `${failed.length} 项授权失败`
    notifyError(actionError.value)
  } else {
    success('授权成功')
  }
  grantDbs.value = []
  grantPerms.value = []
  await selectUser(selectedUser.value)
}

async function revokeAllPermissions(db: string, perms: string[]) {
  if (!selectedUser.value) return
  actionError.value = ''
  try {
    const results = await Promise.allSettled(perms.map((perm) =>
      apiDelete(`/api/v1/users/${encodeURIComponent(selectedUser.value!.name)}/database-permissions/${encodeURIComponent(db)}/${perm}`),
    ))
    const failed = results.filter((r) => r.status === 'rejected').length
    if (failed) {
      actionError.value = `${failed} 项撤销失败`
      notifyError(actionError.value)
    } else {
      success('权限已撤销')
    }
    await selectUser(selectedUser.value)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '撤销失败'
    notifyError(actionError.value)
  }
}

async function revokePermission(database: string, permission: string) {
  if (!selectedUser.value) return
  try {
    await apiDelete(`/api/v1/users/${encodeURIComponent(selectedUser.value.name)}/database-permissions/${encodeURIComponent(database)}/${permission}`)
    await selectUser(selectedUser.value)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '撤销失败'
  }
}

const groupedGrants = computed(() => {
  const map = new Map<string, string[]>()
  for (const g of userGrants.value) {
    const perms = map.get(g.database) ?? []
    perms.push(g.permission)
    map.set(g.database, perms)
  }
  return [...map.entries()]
})

const isCurrentUserAdmin = computed(() => {
  const self_ = users.value.find((u) => u.name === currentUser.value)
  return self_?.role === 'admin'
})

const filteredUsers = computed(() => {
  if (isCurrentUserAdmin.value) return users.value
  return users.value.filter((u) => u.name === currentUser.value)
})
</script>

<template>
  <div v-if="!isAdmin" class="mx-auto max-w-md space-y-4 rounded-xl border border-slate-200 bg-white p-6">
    <h3 class="text-sm font-semibold text-slate-800">修改我的密码</h3>
    <p class="text-xs text-slate-500">普通用户仅可修改自己的密码。数据库与管理操作请联系管理员授权。</p>
    <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm text-white" @click="showChangeSelfPassword = true">打开修改密码</button>
    <UserModals
      v-model:show-create="showCreate"
      v-model:show-set-password="showSetPassword"
      v-model:show-change-self-password="showChangeSelfPassword"
      v-model:new-user="newUser"
      v-model:set-password-value="setPasswordValue"
      v-model:self-old-password="selfOldPassword"
      v-model:self-new-password="selfNewPassword"
      :set-password-user="setPasswordUser"
      @create-user="createUser"
      @set-password="doSetPassword"
      @change-password="doChangeSelfPassword"
    />
  </div>
  <div v-else class="flex flex-col gap-6 lg:flex-row">
    <div class="w-full shrink-0 lg:w-80">
      <p v-if="loadError" class="mb-3 rounded-lg border border-red-200 bg-red-50 p-2 text-xs text-red-700">{{ loadError }}</p>
      <p v-if="actionError" class="mb-3 rounded-lg border border-red-200 bg-red-50 p-2 text-xs text-red-700">{{ actionError }}</p>
      <div class="mb-3 flex gap-2">
        <button v-if="isCurrentUserAdmin" class="flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700" @click="showCreate = true">
          <Plus class="h-3 w-3 shrink-0" />新建用户
        </button>
        <button
          v-if="isCurrentUserAdmin && selectedUser && selectedUser.name !== currentUser"
          class="flex items-center gap-1 rounded-lg border border-slate-300 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-50"
          @click="setPasswordUser = selectedUser.name; setPasswordValue = ''; showSetPassword = true"
        >
          <Key class="h-3 w-3 shrink-0" />设置密码
        </button>
        <button class="flex items-center gap-1 rounded-lg border border-slate-300 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-50" @click="showChangeSelfPassword = true">
          <Lock class="h-3 w-3 shrink-0" />修改密码
        </button>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white">
        <div v-if="!filteredUsers.length" class="p-4 text-center text-xs text-slate-400">暂无用户</div>
        <div v-for="user in filteredUsers" :key="user.name" :class="['flex items-center justify-between border-b border-slate-100 px-4 py-2.5 last:border-b-0 cursor-pointer hover:bg-slate-50', selectedUser?.name === user.name ? 'bg-slate-50' : '']" @click="selectUser(user)">
          <div>
            <p class="text-sm font-medium text-slate-700">{{ user.display_name || user.name }}</p>
            <p class="text-xs" :class="user.disabled ? 'text-red-500' : 'text-slate-400'">
              {{ user.role === 'admin' ? '管理员' : '普通用户' }}<span v-if="user.disabled"> · 已禁用</span>
            </p>
          </div>
          <div class="flex items-center gap-1">
            <button
              v-if="user.name === currentUser"
              class="rounded p-0.5 text-slate-400 hover:text-slate-600"
              title="修改密码"
              @click.stop="showChangeSelfPassword = true"
            >
              <Lock class="h-3 w-3" />
            </button>
            <button
              v-else
              class="rounded p-0.5 text-slate-400 hover:text-slate-600"
              title="设置密码"
              @click.stop="setPasswordUser = user.name; setPasswordValue = ''; showSetPassword = true"
            >
              <Key class="h-3 w-3" />
            </button>
            <button class="rounded p-0.5 text-xs text-slate-400 hover:text-slate-600" :title="user.disabled ? '启用' : '禁用'" @click.stop="toggleDisable(user)">{{ user.disabled ? '启用' : '禁用' }}</button>
            <button class="rounded p-0.5 text-slate-400 hover:text-red-600" @click.stop="deleteUser(user.name)"><Trash2 class="h-3.5 w-3.5" /></button>
          </div>
        </div>
      </div>
    </div>
    <div class="flex-1 min-w-0">
      <div v-if="!selectedUser" class="flex h-48 items-center justify-center rounded-xl border border-dashed border-slate-300 bg-white text-sm text-slate-400">选择用户查看数据库权限</div>
      <div v-else class="space-y-4">
        <!-- 已有权限 -->
        <div class="rounded-xl border border-slate-200 bg-white">
          <div class="border-b border-slate-100 px-5 py-3">
            <h3 class="text-sm font-semibold text-slate-800">
              {{ selectedUser.display_name || selectedUser.name }} 的数据库权限
            </h3>
          </div>
          <div v-if="!userGrants.length" class="px-5 py-8 text-center text-sm text-slate-400">暂无已授权权限</div>
          <table v-else class="w-full text-sm">
            <thead>
              <tr class="border-b border-slate-100 text-left">
                <th class="px-5 py-2 text-xs font-medium text-slate-500 w-40">数据库</th>
                <th class="px-5 py-2 text-xs font-medium text-slate-500">权限</th>
                <th class="px-5 py-2 text-xs font-medium text-slate-500 w-20">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="[db, perms] in groupedGrants" :key="db" class="border-b border-slate-50 hover:bg-slate-50">
                <td class="px-5 py-2.5 font-medium text-slate-700">{{ db }}</td>
                <td class="px-5 py-2.5">
                  <div class="flex flex-wrap gap-1.5">
                    <span
                      v-for="perm in perms"
                      :key="perm"
                      :class="perm === 'admin' ? 'bg-red-50 text-red-700 border-red-200' : perm === 'write' ? 'bg-amber-50 text-amber-700 border-amber-200' : 'bg-blue-50 text-blue-700 border-blue-200'"
                      class="inline-flex items-center gap-1 rounded border px-2 py-0.5 text-xs font-medium"
                    >
                      {{ perm }}
                      <button class="ml-0.5 rounded-full p-0.5 hover:bg-black/10" title="撤销" @click="revokePermission(db, perm)">
                        <X class="h-3 w-3" />
                      </button>
                    </span>
                  </div>
                </td>
                <td class="px-5 py-2.5">
                  <button class="rounded p-1 text-xs text-slate-400 hover:bg-red-50 hover:text-red-600" @click="revokeAllPermissions(db, perms)">全部撤销</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- 新增授权 -->
        <div class="rounded-xl border border-slate-200 bg-white p-5">
          <h3 class="mb-4 text-sm font-semibold text-slate-800">新增授权</h3>
          <div class="mb-4">
            <p class="mb-2 text-xs font-medium text-slate-500">选择数据库</p>
            <div v-if="!databases.length" class="text-xs text-slate-400">暂无数据库，请先创建数据库</div>
            <div v-else class="flex flex-wrap gap-2">
              <label
                v-for="db in databases"
                :key="db"
                :class="[
                  'flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors select-none',
                  grantDbs.includes(db)
                    ? 'border-slate-400 bg-slate-100 text-slate-900'
                    : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300 hover:bg-slate-50',
                ]"
              >
                <div :class="['flex h-4 w-4 items-center justify-center rounded border-2 transition-colors', grantDbs.includes(db) ? 'border-slate-700 bg-slate-700' : 'border-slate-300']">
                  <svg v-if="grantDbs.includes(db)" class="h-3 w-3 text-white" viewBox="0 0 12 12" fill="none"><path d="M2.5 6L5 8.5L9.5 3.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
                </div>
                <input type="checkbox" :checked="grantDbs.includes(db)" class="sr-only" @change="toggleGrantDb(db)" />
                {{ db }}
              </label>
            </div>
          </div>
          <div class="mb-4">
            <p class="mb-2 text-xs font-medium text-slate-500">选择权限</p>
            <div class="flex flex-wrap gap-2">
              <label
                v-for="perm in (['read', 'write', 'admin'] as const)"
                :key="perm"
                :class="[
                  'flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors select-none',
                  grantPerms.includes(perm)
                    ? perm === 'admin' ? 'border-red-300 bg-red-50 text-red-700' : perm === 'write' ? 'border-amber-300 bg-amber-50 text-amber-700' : 'border-blue-300 bg-blue-50 text-blue-700'
                    : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300 hover:bg-slate-50',
                ]"
              >
                <div :class="['flex h-4 w-4 items-center justify-center rounded border-2 transition-colors', grantPerms.includes(perm) ? 'border-slate-700 bg-slate-700' : 'border-slate-300']">
                  <svg v-if="grantPerms.includes(perm)" class="h-3 w-3 text-white" viewBox="0 0 12 12" fill="none"><path d="M2.5 6L5 8.5L9.5 3.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
                </div>
                <input type="checkbox" :checked="grantPerms.includes(perm)" class="sr-only" @change="toggleGrantPerm(perm)" />
                <span class="text-xs font-medium">{{ perm }}</span>
                <span class="text-[10px] opacity-60">{{ perm === 'admin' ? '(读写+管理)' : perm === 'write' ? '(写入数据)' : '(读取数据)' }}</span>
              </label>
            </div>
          </div>
          <div class="flex justify-end">
            <button
              :disabled="!grantDbs.length || !grantPerms.length"
              class="flex items-center gap-2 rounded-lg bg-slate-800 px-5 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-40"
              @click="grantPermission"
            >
              <Shield class="h-4 w-4" />授予权限
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
  <UserModals
    v-model:show-create="showCreate"
    v-model:show-set-password="showSetPassword"
    v-model:show-change-self-password="showChangeSelfPassword"
    v-model:new-user="newUser"
    v-model:set-password-value="setPasswordValue"
    v-model:self-old-password="selfOldPassword"
    v-model:self-new-password="selfNewPassword"
    :set-password-user="setPasswordUser"
    :current-user="currentUser"
    @create-user="createUser"
    @set-password="doSetPassword"
    @change-password="doChangeSelfPassword"
  />
</template>
