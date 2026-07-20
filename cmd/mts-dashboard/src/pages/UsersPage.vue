<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { apiGet, apiPost, apiPut, apiDelete } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { Plus, Trash2, Key, Lock, Download } from 'lucide-vue-next'
import UserModals from '@/components/UserModals.vue'
import UserGrantPanel from '@/components/UserGrantPanel.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { formatMessage } from '@/utils/formatMessage'
import { filterUsers } from '@/utils/listFilter'
import { buildUsersExport, usersToCSV } from '@/utils/usersExport'
import { downloadJSON, downloadText, stampFilename } from '@/utils/download'

interface User { name: string; display_name?: string; role?: string; disabled?: boolean; metadata?: Record<string, string> }
interface UsersResponse { users: User[] }
interface DatabaseGrant { database: string; permission: string }
interface PermissionsResponse { grants: DatabaseGrant[] }

const users = ref<User[]>([])
const userFilter = ref('')
const roleFilter = ref('')
const filteredUsers = computed(() => filterUsers(users.value, userFilter.value, roleFilter.value))
const databases = ref<string[]>([])
const { currentUser, isAdmin } = useAuth()
const { t } = useI18n()
function roleLabel(role?: string): string {
  if (role === 'admin') return t.value('roleAdmin')
  return t.value('roleUser')
}
const { success, error: notifyError, warn } = useNotify()
const loadError = ref('')
const actionResult = ref<ActionResult | null>(null)
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
    loadError.value = formatCaughtError(e)
  }
}

async function createUser() {
  if (!newUser.value.name.trim()) return
  actionResult.value = null
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
    actionResult.value = makeActionResult('ok', t.value('usersCreated'))
    success(t.value('usersCreated'))
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}

async function doSetPassword() {
  if (!setPasswordValue.value) return
  actionResult.value = null
  try {
    await apiPut(`/api/v1/users/${encodeURIComponent(setPasswordUser.value)}/password`, { password: setPasswordValue.value })
    showSetPassword.value = false
    setPasswordValue.value = ''
    actionResult.value = makeActionResult('ok', t.value('usersPasswordSet'))
    success(t.value('usersPasswordSet'))
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}

async function doChangeSelfPassword() {
  if (!selfOldPassword.value || !selfNewPassword.value) return
  actionResult.value = null
  try {
    await apiPost('/api/v1/auth/password', {
      user_name: currentUser.value,
      old_password: selfOldPassword.value,
      new_password: selfNewPassword.value,
    })
    showChangeSelfPassword.value = false
    selfOldPassword.value = ''
    selfNewPassword.value = ''
    actionResult.value = makeActionResult('ok', t.value('usersPasswordChanged'))
    success(t.value('usersPasswordChanged'))
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
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
    const okMsg = formatMessage(t.value('usersDeleted'), { name })
    actionResult.value = makeActionResult('ok', okMsg)
    success(okMsg)
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  } finally {
    deleteLoading.value = false
  }
}

async function toggleDisable(user: User) {
  try {
    await apiPut(`/api/v1/users/${encodeURIComponent(user.name)}`, { ...user, disabled: !user.disabled })
    await loadUsers()
    const okMsg = user.disabled ? t.value('usersEnabled') : t.value('usersDisabled')
    actionResult.value = makeActionResult('ok', okMsg)
    success(okMsg)
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}

async function selectUser(user: User) {
  selectedUser.value = user
  try {
    const data = await apiGet<PermissionsResponse>(`/api/v1/users/${encodeURIComponent(user.name)}/database-permissions`)
    userGrants.value = data.grants ?? []
  } catch (e) {
    const msg = formatCaughtError(e); actionResult.value = makeActionResult('error', msg)
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
  actionResult.value = null
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
    actionResult.value = makeActionResult('ok', t.value('usersGrantOk'))
    success(t.value('usersGrantOk'))
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}

async function revokeGrant(g: DatabaseGrant) {
  if (!selectedUser.value) return
  try {
    await apiDelete(`/api/v1/users/${encodeURIComponent(selectedUser.value.name)}/database-permissions/${encodeURIComponent(g.database)}/${encodeURIComponent(g.permission)}`)
    await selectUser(selectedUser.value)
    actionResult.value = makeActionResult('ok', t.value('usersRevokeOk'))
    success(t.value('usersRevokeOk'))
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}

function openSetPassword(name: string) {
  setPasswordUser.value = name
  setPasswordValue.value = ''
  showSetPassword.value = true
}

function exportJSON() {
  if (!filteredUsers.value.length) {
    warn(t.value('inventoryExportEmpty'))
    return
  }
  downloadJSON(stampFilename('mts-users', 'json'), buildUsersExport(filteredUsers.value))
  success(t.value('inventoryExported'))
}

function exportCSV() {
  if (!filteredUsers.value.length) {
    warn(t.value('inventoryExportEmpty'))
    return
  }
  downloadText(stampFilename('mts-users', 'csv'), usersToCSV(filteredUsers.value), 'text/csv;charset=utf-8')
  success(t.value('inventoryExported'))
}
</script>

<template>
  <div class="space-y-6" data-testid="users-page">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">{{ t('usersTitle') }}</h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('usersDesc') }}</p>
      </div>
      <div class="flex gap-2">
        <button type="button" class="mts-btn" data-testid="users-export-json" :disabled="!filteredUsers.length" @click="exportJSON">
          <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="users-export-csv" :disabled="!filteredUsers.length" @click="exportCSV">
          <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportCSV') }}
        </button>
        <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900 px-3 py-1.5 text-xs text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800" @click="showChangeSelfPassword = true">
          <Lock class="h-3.5 w-3.5" /> {{ t('usersChangeMyPassword') }}
        </button>
        <button v-if="isAdmin" class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700" @click="showCreate = true">
          <Plus class="h-3.5 w-3.5" /> {{ t('usersCreate') }}
        </button>
      </div>
    </div>

    <ActionResultBanner
      v-if="loadError"
      kind="error"
      :message="loadError"
      @dismiss="loadError = ''"
    />
    <ActionResultBanner :result="actionResult" @dismiss="actionResult = null" />

    <div v-if="!isAdmin" class="rounded-xl border border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-900/60 p-4 text-sm text-slate-600 dark:text-slate-300">
      {{ t('usersNonAdminHint') }}
    </div>

    <template v-else>
    <div class="flex flex-wrap items-end gap-3" data-testid="users-filter-bar">
      <label class="text-xs mts-muted">{{ t('filter') }}
        <input
          v-model="userFilter"
          type="search"
          class="mts-input mt-1 min-w-[12rem]"
          data-testid="users-filter"
          :placeholder="t('usersFilterPlaceholder')"
        />
      </label>
      <label class="text-xs mts-muted">{{ t('accountRole') }}
        <select v-model="roleFilter" class="mts-input mt-1" data-testid="users-role-filter">
          <option value="">{{ t('usersAllRoles') }}</option>
          <option value="admin">{{ t('roleAdmin') }}</option>
          <option value="user">{{ t('roleUser') }}</option>
        </select>
      </label>
      <span class="text-xs mts-muted" data-testid="users-filter-count">{{ filteredUsers.length }} / {{ users.length }}</span>
    </div>

    <div v-if="!filteredUsers.length" class="mts-card">
      <EmptyState
        data-testid="users-empty-filter"
        :title="users.length ? t('usersFilterEmpty') : t('usersEmpty')"
        :description="users.length ? t('usersFilterEmptyDesc') : t('usersEmptyDesc')"
      >
        <template v-if="!users.length" #action>
          <button type="button" class="mts-btn-primary" @click="showCreate = true">{{ t('usersCreate') }}</button>
        </template>
      </EmptyState>
    </div>

    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
    <div class="mts-table-wrap" data-testid="users-table-wrap"><table class="w-full min-w-[40rem] text-sm">
        <thead>
          <tr class="border-b border-slate-100 bg-slate-50/50 text-left text-xs uppercase text-slate-500 dark:text-slate-400 dark:text-slate-500">
            <th class="px-4 py-2.5">{{ t('usersColUser') }}</th>
            <th class="px-4 py-2.5">{{ t('usersColRole') }}</th>
            <th class="px-4 py-2.5">{{ t('usersColStatus') }}</th>
            <th class="px-4 py-2.5">{{ t('action') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in filteredUsers" :key="u.name" class="border-b border-slate-50 hover:bg-slate-50 dark:hover:bg-slate-800/40">
            <td class="px-4 py-3">
              <button class="text-left font-medium text-slate-800 dark:text-slate-100 hover:underline" @click="selectUser(u)">{{ u.name }}</button>
              <span v-if="u.display_name" class="ml-2 text-xs text-slate-400 dark:text-slate-500">{{ u.display_name }}</span>
            </td>
            <td class="px-4 py-3 text-slate-600 dark:text-slate-300">{{ roleLabel(u.role) }}</td>
            <td class="px-4 py-3">
              <span :class="u.disabled ? 'bg-red-50 text-red-700 dark:text-red-200' : 'bg-green-50 text-green-700 dark:text-green-200'" class="rounded px-2 py-0.5 text-xs">{{ u.disabled ? t('usersStatusDisabled') : t('usersStatusActive') }}</span>
            </td>
            <td class="px-4 py-3">
              <div class="flex items-center gap-1">
                <button class="rounded p-1 text-slate-400 dark:text-slate-500 hover:text-slate-700 dark:text-slate-200" :title="t('usersSetPassword')" @click="openSetPassword(u.name)"><Key class="h-4 w-4" /></button>
                <button class="rounded px-2 py-0.5 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500 hover:bg-slate-100 dark:bg-slate-800" @click="toggleDisable(u)">{{ u.disabled ? t('usersEnable') : t('usersDisable') }}</button>
                <button class="rounded p-1 text-slate-400 dark:text-slate-500 hover:text-red-600 dark:text-red-300" @click="requestDelete(u.name)"><Trash2 class="h-4 w-4" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table></div>
    </div>

    </template>

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
      :title="t('usersDeleteTitle')"
      :message="formatMessage(t('usersDeleteMsg'), { name: deleteName })"
      :confirm-label="t('delete')"
      danger
      :loading="deleteLoading"
      @confirm="confirmDelete"
    />
  </div>
</template>
