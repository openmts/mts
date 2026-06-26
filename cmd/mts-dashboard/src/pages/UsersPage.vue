<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost, apiPut, apiDelete } from '@/api/client'
import { Plus, Trash2, Shield, X } from 'lucide-vue-next'

interface User { name: string; display_name?: string; disabled?: boolean }
interface UsersResponse { users: User[] }
interface DatabaseGrant { database: string; permission: string }
interface PermissionsResponse { grants: DatabaseGrant[] }

const users = ref<User[]>([])
const loadError = ref('')
const actionError = ref('')
const showCreate = ref(false)
const newUser = ref({ name: '', display_name: '' })
const selectedUser = ref<User | null>(null)
const userGrants = ref<DatabaseGrant[]>([])
const grantDb = ref('')
const grantPerm = ref<'read' | 'write' | 'admin'>('read')

onMounted(loadUsers)

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
    await apiPost('/api/v1/users', { name: newUser.value.name.trim(), display_name: newUser.value.display_name })
    showCreate.value = false
    newUser.value = { name: '', display_name: '' }
    await loadUsers()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建失败'
  }
}

async function deleteUser(name: string) {
  if (!confirm(`确定删除用户 ${name}？`)) return
  try {
    await apiDelete(`/api/v1/users/${name}`)
    await loadUsers()
    if (selectedUser.value?.name === name) selectedUser.value = null
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
  }
}

async function toggleDisable(user: User) {
  try {
    await apiPut(`/api/v1/users/${user.name}`, { ...user, disabled: !user.disabled })
    await loadUsers()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '操作失败'
  }
}

async function selectUser(user: User) {
  selectedUser.value = user
  try {
    const data = await apiGet<PermissionsResponse>(`/api/v1/users/${user.name}/database-permissions`)
    userGrants.value = data.grants ?? []
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '加载权限失败'
  }
}

async function grantPermission() {
  if (!grantDb.value.trim() || !selectedUser.value) return
  try {
    await apiPut(`/api/v1/users/${selectedUser.value.name}/database-permissions/${grantDb.value.trim()}/${grantPerm.value}`)
    await selectUser(selectedUser.value)
    grantDb.value = ''
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '授权失败'
  }
}

async function revokePermission(database: string, permission: string) {
  if (!selectedUser.value) return
  try {
    await apiDelete(`/api/v1/users/${selectedUser.value.name}/database-permissions/${database}/${permission}`)
    await selectUser(selectedUser.value)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '撤销失败'
  }
}
</script>

<template>
  <div class="flex gap-6">
    <div class="w-80 shrink-0">
      <p v-if="loadError" class="mb-3 rounded-lg border border-red-200 bg-red-50 p-2 text-xs text-red-700">{{ loadError }}</p>
      <p v-if="actionError" class="mb-3 rounded-lg border border-red-200 bg-red-50 p-2 text-xs text-red-700">{{ actionError }}</p>
      <div class="mb-3 flex gap-2">
        <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700" @click="showCreate = true">
          <Plus class="h-3 w-3" /> 新建用户
        </button>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white">
        <div v-if="!users.length" class="p-4 text-center text-xs text-slate-400">暂无用户</div>
        <div v-for="user in users" :key="user.name" :class="['flex items-center justify-between border-b border-slate-100 px-4 py-2.5 last:border-b-0 cursor-pointer hover:bg-slate-50', selectedUser?.name === user.name ? 'bg-slate-50' : '']" @click="selectUser(user)">
          <div>
            <p class="text-sm font-medium text-slate-700">{{ user.display_name || user.name }}</p>
            <p v-if="user.disabled" class="text-xs text-red-500">已禁用</p>
          </div>
          <div class="flex items-center gap-1">
            <button class="rounded p-0.5 text-xs text-slate-400 hover:text-slate-600" :title="user.disabled ? '启用' : '禁用'" @click.stop="toggleDisable(user)">{{ user.disabled ? '启用' : '禁用' }}</button>
            <button class="rounded p-0.5 text-slate-400 hover:text-red-600" @click.stop="deleteUser(user.name)"><Trash2 class="h-3.5 w-3.5" /></button>
          </div>
        </div>
      </div>
    </div>
    <div class="flex-1">
      <div v-if="!selectedUser" class="flex h-48 items-center justify-center rounded-xl border border-dashed border-slate-300 bg-white text-sm text-slate-400">选择左侧用户查看数据库权限</div>
      <div v-else class="rounded-xl border border-slate-200 bg-white p-6">
        <h3 class="mb-4 text-sm font-semibold text-slate-800">{{ selectedUser.display_name || selectedUser.name }} 的数据库权限</h3>
        <div v-if="!userGrants.length" class="mb-4 text-xs text-slate-400">暂无权限</div>
        <div v-else class="mb-4 space-y-1">
          <div v-for="grant in userGrants" :key="`${grant.database}-${grant.permission}`" class="flex items-center justify-between rounded bg-slate-50 px-3 py-1.5">
            <span class="text-sm text-slate-700">
              <span class="font-medium">{{ grant.database }}</span>
              <span class="ml-2 rounded bg-slate-200 px-1.5 py-0.5 text-xs text-slate-600">{{ grant.permission }}</span>
            </span>
            <button class="text-slate-400 hover:text-red-600" @click="revokePermission(grant.database, grant.permission)"><X class="h-3.5 w-3.5" /></button>
          </div>
        </div>
        <div class="flex gap-2">
          <input v-model="grantDb" type="text" placeholder="数据库名" class="w-40 rounded-lg border border-slate-300 px-3 py-1.5 text-xs focus:border-slate-500 focus:outline-none" />
          <select v-model="grantPerm" class="rounded-lg border border-slate-300 px-2 py-1.5 text-xs focus:border-slate-500 focus:outline-none">
            <option value="read">read</option>
            <option value="write">write</option>
            <option value="admin">admin</option>
          </select>
          <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700" @click="grantPermission">
            <Shield class="h-3 w-3" /> 授权
          </button>
        </div>
      </div>
    </div>
  </div>
  <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showCreate = false">
    <div class="w-80 rounded-xl bg-white p-6 shadow-lg">
      <h3 class="mb-4 text-sm font-semibold text-slate-800">创建用户</h3>
      <div class="space-y-3">
        <input v-model="newUser.name" type="text" placeholder="用户名 (必填)" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" @keyup.enter="createUser" />
        <input v-model="newUser.display_name" type="text" placeholder="显示名 (可选)" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" />
      </div>
      <div class="mt-4 flex justify-end gap-2">
        <button class="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100" @click="showCreate = false">取消</button>
        <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="createUser">创建</button>
      </div>
    </div>
  </div>
</template>
