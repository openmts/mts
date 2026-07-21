<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { apiGet, apiPost, apiPut, apiDelete } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { useMutationGuard } from '@/composables/useMutationGuard'
import {
  isPasswordDraftDirty,
  isUserCreateDraftDirty,
  shouldBlockLeaveAdminCreate,
} from '@/utils/adminFormDirty'
import { registerDirtyChecker } from '@/utils/routeDirty'
import { Plus, Trash2, Key, Lock, Download } from 'lucide-vue-next'
import UserModals from '@/components/UserModals.vue'
import UserGrantPanel from '@/components/UserGrantPanel.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { formatMessage } from '@/utils/formatMessage'
import { filterUsers } from '@/utils/listFilter'
import { filterRowsByIds } from '@/utils/listSelection'
import { useListSelection } from '@/composables/useListSelection'
import {
  cycleSortState,
  loadSortState,
  saveSortState,
  sortByAccessor,
  type SortState,
} from '@/utils/listSort'
import { USERS_CSV_HEADER, buildUsersExport, userToCSVLine, usersToCSV } from '@/utils/usersExport'
import { parseUsersPrefill, usersFormToPrefill } from '@/utils/routePrefill'
import { copyText } from '@/utils/clipboard'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'

interface User { name: string; display_name?: string; role?: string; disabled?: boolean; metadata?: Record<string, string> }
interface UsersResponse { users: User[] }
interface DatabaseGrant { database: string; permission: string }
interface PermissionsResponse { grants: DatabaseGrant[] }

const route = useRoute()
useHashScroll()
const users = ref<User[]>([])
const userFilter = ref('')
const roleFilter = ref('')
const USERS_SORT_KEY = 'mts.dashboard.users-sort.prefs.v1'
const USERS_SORT_KEYS = ['name', 'role', 'status'] as const
type UserSortKey = (typeof USERS_SORT_KEYS)[number]
const storage = typeof localStorage !== 'undefined' ? localStorage : null
const userSort = ref<SortState<UserSortKey>>(loadSortState(storage, USERS_SORT_KEY, USERS_SORT_KEYS))

const filteredUsers = computed(() => {
  const base = filterUsers(users.value, userFilter.value, roleFilter.value)
  return sortByAccessor(base, userSort.value, {
    name: (u) => u.name,
    role: (u) => u.role || '',
    status: (u) => Boolean(u.disabled),
  })
})
const visibleUserIds = computed(() => filteredUsers.value.map((u) => u.name))
const USERS_ROW_HEIGHT = 52
const USERS_LIST_HEIGHT = 448

function cycleUserSort(key: UserSortKey) {
  userSort.value = cycleSortState(userSort.value, key)
  saveSortState(storage, USERS_SORT_KEY, userSort.value)
}

function userSortIndicator(key: UserSortKey): string {
  if (userSort.value.key !== key) return ''
  return userSort.value.dir === 'asc' ? '↑' : '↓'
}
const {
  selectedIds,
  selectedCount,
  allVisibleSelected,
  someVisibleSelected,
  exportIds,
  isSelected,
  toggle,
  toggleAllVisible,
  clear: clearSelection,
  pruneTo,
} = useListSelection(visibleUserIds)
const databases = ref<string[]>([])
const { currentUser, isAdmin } = useAuth()
const { offline, writeBlocked, blockReason, blockedMessageKey } = useMutationGuard()
const { t } = useI18n()
function roleLabel(role?: string): string {
  if (role === 'admin') return t.value('roleAdmin')
  return t.value('roleUser')
}
const { success, error: notifyError, warn } = useNotify()
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  runTextExport,
  runJSONExport,
} = useExportJob()
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
const batchOpen = ref(false)
const batchMode = ref<'enable' | 'disable'>('enable')
const batchLoading = ref(false)

onMounted(async () => {
  unregisterUsersDirty = registerDirtyChecker('users', () => usersFormDirty.value)
  window.addEventListener('beforeunload', onUsersBeforeUnload)

  if (!isAdmin.value) return
  await loadUsers()
  try {
    const { listDatabases } = await import('@/api/meta')
    databases.value = await listDatabases()
  } catch (_) { /* 非关键 */ }
  await applyUsersPrefillFromRoute()
})

watch(
  () => route.fullPath,
  (path, prev) => {
    if (!isAdmin.value) return
    if (prev != null && path !== prev) void applyUsersPrefillFromRoute()
  },
)

async function applyUsersPrefillFromRoute() {
  if (!isAdmin.value) return
  const pre = parseUsersPrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.q != null && userFilter.value !== pre.q) {
    userFilter.value = pre.q
    changed = true
  }
  if (pre.role != null && roleFilter.value !== pre.role) {
    roleFilter.value = pre.role
    changed = true
  }
  if (pre.user) {
    const u = users.value.find((x) => x.name === pre.user)
    if (u && selectedUser.value?.name !== u.name) {
      await selectUser(u)
      changed = true
    }
  }
  if (changed) success(t.value('usersPrefillApplied'))
}

async function copyUsersShareLink() {
  const path = usersFormToPrefill({
    q: userFilter.value,
    role: roleFilter.value || undefined,
    user: selectedUser.value?.name,
  }, { hash: selectedUser.value ? '#user-grant-panel' : '#users-filter-bar' })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('usersShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

async function loadUsers() {
  try {
    const data = await apiGet<UsersResponse>('/api/v1/users')
    users.value = data.users ?? []
    pruneTo(users.value.map((u) => u.name))
  } catch (e) {
    loadError.value = formatCaughtError(e)
  }
}

const usersFormDirty = computed(() => {
  if (shouldBlockLeaveAdminCreate(showCreate.value, isUserCreateDraftDirty(newUser.value))) return true
  if (showSetPassword.value && isPasswordDraftDirty(setPasswordValue.value)) return true
  if (
    showChangeSelfPassword.value &&
    isPasswordDraftDirty(selfNewPassword.value, selfOldPassword.value)
  ) {
    return true
  }
  return false
})

function onUsersBeforeUnload(e: BeforeUnloadEvent) {
  if (!usersFormDirty.value) return
  e.preventDefault()
  e.returnValue = ''
}

let unregisterUsersDirty: (() => void) | null = null

async function createUser() {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
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
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
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
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
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
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineAdminBlocked')))
    return
  }
  deleteName.value = name
  deleteOpen.value = true
}

async function confirmDelete() {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
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
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
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
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
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
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
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
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineAdminBlocked')))
    return
  }
  setPasswordUser.value = name
  setPasswordValue.value = ''
  showSetPassword.value = true
}

function rowsForExport() {
  return filterRowsByIds(filteredUsers.value, exportIds.value, (u) => u.name)
}

async function exportJSON() {
  const rows = rowsForExport()
  if (!rows.length) {
    warn(t.value('inventoryExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-users', 'json'),
    total: rows.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, rows.length)
      const chunk = 400
      for (let i = 0; i < rows.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === rows.length || done % chunk === 0) {
          progress(done, rows.length)
          if (done < rows.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return buildUsersExport(rows)
    },
  })
  if (outcome === 'done') success(t.value('inventoryExported'))
  else if (outcome === 'cancelled') success(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function exportCSV() {
  const rows = rowsForExport()
  if (!rows.length) {
    warn(t.value('inventoryExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const outcome = await runTextExport({
    label: 'CSV',
    filename: stampFilename('mts-users', 'csv'),
    mime: 'text/csv;charset=utf-8',
    total: rows.length,
    build: async ({ isCancelled, progress }) => {
      const lines = [USERS_CSV_HEADER]
      progress(0, rows.length)
      const chunk = 400
      for (let i = 0; i < rows.length; i++) {
        if (isCancelled()) return null
        lines.push(userToCSVLine(rows[i]))
        const done = i + 1
        if (done === rows.length || done % chunk === 0) {
          progress(done, rows.length)
          if (done < rows.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return lines.join('\n')
    },
  })
  if (outcome === 'done') success(t.value('inventoryExported'))
  else if (outcome === 'cancelled') success(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

function openBatch(mode: 'enable' | 'disable') {
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineAdminBlocked')))
    return
  }
  if (!selectedIds.value.length) return
  batchMode.value = mode
  batchOpen.value = true
}

async function confirmBatch() {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  const names = selectedIds.value.slice()
  if (!names.length) {
    batchOpen.value = false
    return
  }
  batchLoading.value = true
  let ok = 0
  let skip = 0
  let fail = 0
  const wantDisabled = batchMode.value === 'disable'
  try {
    for (const name of names) {
      const user = users.value.find((u) => u.name === name)
      if (!user) {
        skip += 1
        continue
      }
      if (wantDisabled && name === currentUser.value) {
        skip += 1
        continue
      }
      if (Boolean(user.disabled) === wantDisabled) {
        skip += 1
        continue
      }
      try {
        await apiPut(`/api/v1/users/${encodeURIComponent(user.name)}`, {
          ...user,
          disabled: wantDisabled,
        })
        ok += 1
      } catch {
        fail += 1
      }
    }
    await loadUsers()
    const key = fail ? 'listBatchPartial' : 'listBatchDone'
    const msg = formatMessage(t.value(key), { ok, skip, fail })
    actionResult.value = makeActionResult(fail ? 'error' : 'ok', msg)
    if (fail) notifyError(msg)
    else success(msg)
    clearSelection()
    batchOpen.value = false
  } finally {
    batchLoading.value = false
  }
}
onBeforeUnmount(() => {
  unregisterUsersDirty?.()
  unregisterUsersDirty = null
  window.removeEventListener('beforeunload', onUsersBeforeUnload)
})
</script>

<template>
  <div class="space-y-6" data-testid="users-page">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">{{ t('usersTitle') }}</h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('usersDesc') }}</p>
      </div>
      <div class="flex gap-2">
        <ExportJobBanner class="w-full basis-full" :job="exportJob" @cancel="cancelExport" @dismiss="resetExport" />
        <button type="button" class="mts-btn" data-testid="users-export-json" :disabled="exportBusy || !filteredUsers.length" @click="exportJSON">
          <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="users-export-csv" :disabled="exportBusy || !filteredUsers.length" @click="exportCSV">
          <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportCSV') }}
        </button>
        <button type="button" class="mts-btn" data-testid="users-share-link" @click="copyUsersShareLink">
          {{ t('usersShareLink') }}
        </button>
        <button type="button" class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900 px-3 py-1.5 text-xs text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50" data-testid="users-change-self-open" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="showChangeSelfPassword = true">
          <Lock class="h-3.5 w-3.5" /> {{ t('usersChangeMyPassword') }}
        </button>
        <button v-if="isAdmin" type="button" class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-50" data-testid="users-create-open" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="showCreate = true">
          <Plus class="h-3.5 w-3.5" /> {{ t('usersCreate') }}
        </button>
        <span
          v-if="usersFormDirty"
          data-testid="users-dirty-badge"
          class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
        >{{ t('adminDirtyBadge') }}</span>
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
    <div id="users-filter-bar" class="scroll-mt-20 flex flex-wrap items-end gap-3" data-testid="users-filter-bar">
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
      <ListSelectionToolbar
        prefix="users"
        :selected-count="selectedCount"
        :has-visible="!!filteredUsers.length"
        @select-all="toggleAllVisible(true)"
        @clear="clearSelection"
      >
        <template #actions>
          <button type="button" class="mts-btn" data-testid="users-batch-enable" :disabled="!selectedCount || writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="openBatch('enable')">{{ t('listBatchEnable') }}</button>
          <button type="button" class="mts-btn" data-testid="users-batch-disable" :disabled="!selectedCount || writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="openBatch('disable')">{{ t('listBatchDisable') }}</button>
        </template>
      </ListSelectionToolbar>
    </div>

    <div v-if="!filteredUsers.length" class="mts-card">
      <EmptyState
        data-testid="users-empty-filter"
        :title="users.length ? t('usersFilterEmpty') : t('usersEmpty')"
        :description="users.length ? t('usersFilterEmptyDesc') : t('usersEmptyDesc')"
      >
        <template v-if="!users.length" #action>
          <button type="button" class="mts-btn-primary" data-testid="users-empty-create" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="showCreate = true">{{ t('usersCreate') }}</button>
        </template>
      </EmptyState>
    </div>

    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
        <div class="overflow-x-auto" data-testid="users-table-wrap">
      <div
        id="users-table"
        class="min-w-[40rem] overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800"
        data-testid="users-table"
      >
        <div
          class="grid grid-cols-[2.5rem_minmax(10rem,1.4fr)_minmax(5rem,0.7fr)_minmax(5rem,0.7fr)_minmax(9rem,1fr)] border-b border-slate-100 bg-slate-50/95 text-left text-[11px] font-medium text-slate-500 dark:border-slate-800 dark:bg-slate-900/95 dark:text-slate-400"
          data-testid="users-table-header"
        >
          <div class="px-3 py-2.5">
            <input
              type="checkbox"
              class="h-3.5 w-3.5"
              data-testid="users-select-all-checkbox"
              :checked="allVisibleSelected"
              :indeterminate.prop="someVisibleSelected"
              :aria-label="t('listSelectAll')"
              @change="toggleAllVisible(($event.target as HTMLInputElement).checked)"
            />
          </div>
          <div class="px-4 py-2.5">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="users-sort-name" @click="cycleUserSort('name')">
              {{ t('usersColUser') }} <span aria-hidden="true">{{ userSortIndicator('name') }}</span>
            </button>
          </div>
          <div class="px-4 py-2.5">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="users-sort-role" @click="cycleUserSort('role')">
              {{ t('usersColRole') }} <span aria-hidden="true">{{ userSortIndicator('role') }}</span>
            </button>
          </div>
          <div class="px-4 py-2.5">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="users-sort-status" @click="cycleUserSort('status')">
              {{ t('usersColStatus') }} <span aria-hidden="true">{{ userSortIndicator('status') }}</span>
            </button>
          </div>
          <div class="px-4 py-2.5 uppercase">{{ t('action') }}</div>
        </div>
        <VirtualTable
          :items="filteredUsers"
          :row-height="USERS_ROW_HEIGHT"
          :height="USERS_LIST_HEIGHT"
          data-testid="users-virtual-list"
        >
          <template #default="{ item: u }">
            <div
              class="grid h-full grid-cols-[2.5rem_minmax(10rem,1.4fr)_minmax(5rem,0.7fr)_minmax(5rem,0.7fr)_minmax(9rem,1fr)] items-center border-b border-slate-50 hover:bg-slate-50 dark:hover:bg-slate-800/40"
              :data-testid="`users-row-${u.name}`"
            >
              <div class="px-3">
                <input
                  type="checkbox"
                  class="h-3.5 w-3.5"
                  :data-testid="`users-select-${u.name}`"
                  :checked="isSelected(u.name)"
                  :aria-label="t('listSelectCol') + ' ' + u.name"
                  @change="toggle(u.name, ($event.target as HTMLInputElement).checked)"
                />
              </div>
              <div class="min-w-0 px-4">
                <button type="button" class="text-left font-medium text-slate-800 hover:underline dark:text-slate-100" :data-testid="`users-open-grant-${u.name}`" @click="selectUser(u)">{{ u.name }}</button>
                <span v-if="u.display_name" class="ml-2 text-xs text-slate-400 dark:text-slate-500">{{ u.display_name }}</span>
              </div>
              <div class="px-4 text-slate-600 dark:text-slate-300">{{ roleLabel(u.role) }}</div>
              <div class="px-4">
                <span
                  :class="u.disabled ? 'bg-red-50 text-red-700 dark:text-red-200' : 'bg-green-50 text-green-700 dark:text-green-200'"
                  class="rounded px-2 py-0.5 text-xs"
                >{{ u.disabled ? t('usersStatusDisabled') : t('usersStatusActive') }}</span>
              </div>
              <div class="px-4">
                <div class="flex items-center gap-1">
                  <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-700 dark:text-slate-500 dark:hover:text-slate-200 disabled:cursor-not-allowed disabled:opacity-40" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : t('usersSetPassword')" :disabled="writeBlocked" :data-testid="`users-set-password-${u.name}`" @click="openSetPassword(u.name)"><Key class="h-4 w-4" /></button>
                  <button type="button" class="rounded px-2 py-0.5 text-xs text-slate-500 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" :data-testid="`users-toggle-${u.name}`" @click="toggleDisable(u)">{{ u.disabled ? t('usersEnable') : t('usersDisable') }}</button>
                  <button type="button" class="rounded p-1 text-slate-400 hover:text-red-600 dark:text-slate-500 dark:hover:text-red-300 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" :data-testid="`users-delete-${u.name}`" @click="requestDelete(u.name)"><Trash2 class="h-4 w-4" /></button>
                </div>
              </div>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="users-virtual-hint">
          {{ t('usersVirtualHint') }}
        </p>
      </div>
    </div>
    </div>

    </template>

    <UserGrantPanel
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
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
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
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
      v-model:open="batchOpen"
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
      :offline-message-key="'offlineAdminBlocked'"
      :title="batchMode === 'enable' ? t('listBatchEnableTitle') : t('listBatchDisableTitle')"
      :message="(batchMode === 'enable' ? t('listBatchEnableMsg') : t('listBatchDisableMsg')) + ` (${selectedCount})`"
      :confirm-label="batchMode === 'enable' ? t('listBatchEnable') : t('listBatchDisable')"
      :danger="batchMode === 'disable'"
      :loading="batchLoading"
      @confirm="confirmBatch"
    />
    <ConfirmDialog
      v-model:open="deleteOpen"
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
      :offline-message-key="'offlineAdminBlocked'"
      :title="t('usersDeleteTitle')"
      :message="formatMessage(t('usersDeleteMsg'), { name: deleteName })"
      :confirm-label="t('delete')"
      danger
      :loading="deleteLoading"
      @confirm="confirmDelete"
    />
  </div>
</template>
