<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { apiDelete, apiGet, apiPost, apiPostNDJSONStream, apiPut } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
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
import InFlightBanner from '@/components/InFlightBanner.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import PartialErrorBanner from '@/components/PartialErrorBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { makeActionResult } from '@/utils/actionResult'
import { useActionRetry } from '@/composables/useActionRetry'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError, isCanceledError, isTimeoutError } from '@/utils/apiError'
import { adminHeavyBusyOpFromError, adminOpBusyOpenAction, isAdminHeavyBusyError } from '@/utils/adminOpBusy'
import { createActionAbort } from '@/utils/actionAbort'
import {
  applyBatchProgressEvent,
  batchProgressPercent,
  emptyBatchProgress,
  type BatchProgressState,
  type BatchMutationSummary,
} from '@/utils/batchProgress'
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
  ariaSortValue,
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
const { setAdminOpBusy, refreshAdminOpBusy } = useAdminOpBusy()
const { offline, writeBlocked, blockReason, blockedMessageKey } = useMutationGuard()
const { t } = useI18n()
function roleLabel(role?: string): string {
  if (role === 'admin') return t.value('roleAdmin')
  return t.value('roleUser')
}
const { success, info, error: notifyError, warn } = useNotify()

function notifyMaybeAdminBusy(message: string, err?: unknown) {
  if (err && isAdminHeavyBusyError(err)) {
    setAdminOpBusy(true, adminHeavyBusyOpFromError(err) || undefined)
    void refreshAdminOpBusy()
    notifyError(message, { action: adminOpBusyOpenAction(t.value('adminOpBusyOpenOps')) })
    return
  }
  notifyError(message)
}
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  retryLastExport,
  canRetryExport,
  runTextExport,
  runJSONExport,
} = useExportJob()
const loadError = ref('')
const grantDbError = ref('')
type UsersActionKey = 'create' | 'set-password' | 'change-self-password' | 'delete' | 'toggle' | 'select-grants' | 'grant' | 'revoke' | 'batch'
const {
  lastFailedAction,
  actionResult,
  actionContext,
  canRetryAction,
  clearActionResult,
  setActionOk,
  setActionError,
  setActionResult,
  reportActionError,
} = useActionRetry<UsersActionKey>()
function reportAndNotify(key: UsersActionKey, e: unknown, ctx?: Record<string, string>) {
  reportActionError(key, e, ctx)
  const msg = actionResult.value?.message
  if (msg) notifyMaybeAdminBusy(msg, e)
}
async function retryLastUsersAction() {
  const key = lastFailedAction.value
  if (!key) return
  if (key === 'create') return createUser()
  if (key === 'set-password') return doSetPassword()
  if (key === 'change-self-password') return doChangeSelfPassword()
  if (key === 'delete' && deleteName.value) {
    deleteOpen.value = true
    return confirmDelete()
  }
  if (key === 'toggle' && actionContext.value.name) {
    const u = users.value.find((x) => x.name === actionContext.value.name)
    if (u) return toggleDisable(u)
  }
  if (key === 'select-grants' && selectedUser.value) return selectUser(selectedUser.value)
  if (key === 'grant') return grantPermission()
  if (key === 'revoke' && actionContext.value.database && actionContext.value.permission && selectedUser.value) {
    return revokeGrant({ database: actionContext.value.database, permission: actionContext.value.permission })
  }
  if (key === 'batch') {
    batchOpen.value = true
    return confirmBatch()
  }
}
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
const batchProgress = ref<BatchProgressState>(emptyBatchProgress())
const usersActionStartedAt = ref<number | null>(null)
const usersActionAbort = createActionAbort()
const usersToggleLoading = ref(false)
const usersWriteLoading = ref(false)

onMounted(async () => {
  unregisterUsersDirty = registerDirtyChecker('users', () => usersFormDirty.value)
  window.addEventListener('beforeunload', onUsersBeforeUnload)

  if (!isAdmin.value) return
  await loadUsers()
  await loadGrantDatabases()
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

async function loadGrantDatabases() {
  try {
    const { listDatabases } = await import('@/api/meta')
    databases.value = await listDatabases()
    grantDbError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (databases.value.length) grantDbError.value = msg
    else {
      databases.value = []
      grantDbError.value = msg
    }
  }
}

async function loadUsers() {
  try {
    const data = await apiGet<UsersResponse>('/api/v1/users')
    users.value = data.users ?? []
    pruneTo(users.value.map((u) => u.name))
    loadError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (users.value.length) {
      // soft-keep：刷新失败保留用户列表
      loadError.value = msg
    } else {
      users.value = []
      loadError.value = msg
    }
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
  if (usersWriteLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  if (!newUser.value.name.trim()) return
  clearActionResult()
  usersWriteLoading.value = true
  usersActionStartedAt.value = Date.now()
  const signal = usersActionAbort.begin()
  try {
    await apiPost('/api/v1/users', {
      name: newUser.value.name.trim(),
      display_name: newUser.value.display_name,
      role: newUser.value.role,
      password: newUser.value.password || undefined,
    }, { signal })
    showCreate.value = false
    newUser.value = { name: '', display_name: '', role: 'user', password: '' }
    await loadUsers()
    setActionOk(t.value('usersCreated'))
    success(t.value('usersCreated'))
  } catch (e) {
    reportUsersCatch('create', e)
  } finally {
    usersActionAbort.end()
    usersWriteLoading.value = false
    usersActionStartedAt.value = null
  }
}

async function doSetPassword() {
  if (usersWriteLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  if (!setPasswordValue.value) return
  clearActionResult()
  usersWriteLoading.value = true
  usersActionStartedAt.value = Date.now()
  const signal = usersActionAbort.begin()
  try {
    await apiPut(`/api/v1/users/${encodeURIComponent(setPasswordUser.value)}/password`, { password: setPasswordValue.value }, { signal })
    showSetPassword.value = false
    setPasswordValue.value = ''
    setActionOk(t.value('usersPasswordSet'))
    success(t.value('usersPasswordSet'))
  } catch (e) {
    reportUsersCatch('set-password', e)
  } finally {
    usersActionAbort.end()
    usersWriteLoading.value = false
    usersActionStartedAt.value = null
  }
}

async function doChangeSelfPassword() {
  if (usersWriteLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  if (!selfOldPassword.value || !selfNewPassword.value) return
  clearActionResult()
  usersWriteLoading.value = true
  usersActionStartedAt.value = Date.now()
  const signal = usersActionAbort.begin()
  try {
    await apiPost('/api/v1/auth/password', {
      user_name: currentUser.value,
      old_password: selfOldPassword.value,
      new_password: selfNewPassword.value,
    }, { signal })
    showChangeSelfPassword.value = false
    selfOldPassword.value = ''
    selfNewPassword.value = ''
    setActionOk(t.value('usersPasswordChanged'))
    success(t.value('usersPasswordChanged'))
  } catch (e) {
    reportUsersCatch('change-self-password', e)
  } finally {
    usersActionAbort.end()
    usersWriteLoading.value = false
    usersActionStartedAt.value = null
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

function cancelUsersAction() {
  usersActionAbort.cancel()
}

function reportUsersCatch(key: UsersActionKey, e: unknown, ctx?: Record<string, string>) {
  if (isCanceledError(e)) {
    const msg = t.value('adminActionCancelled')
    setActionResult(makeActionResult('info', msg))
    info(msg)
    return
  }
  if (isTimeoutError(e)) {
    const msg = t.value('adminActionTimedOut')
    setActionResult(makeActionResult('error', msg))
    notifyError(msg)
    return
  }
  reportAndNotify(key, e, ctx)
}

async function confirmDelete() {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  const name = deleteName.value
  if (!name) return
  deleteLoading.value = true
  usersActionStartedAt.value = Date.now()
  const signal = usersActionAbort.begin()
  try {
    await apiDelete(`/api/v1/users/${encodeURIComponent(name)}`, { signal })
    await loadUsers()
    if (selectedUser.value?.name === name) selectedUser.value = null
    deleteOpen.value = false
    const okMsg = formatMessage(t.value('usersDeleted'), { name })
    setActionOk(okMsg)
    success(okMsg)
  } catch (e) {
    reportUsersCatch('delete', e)
  } finally {
    usersActionAbort.end()
    deleteLoading.value = false
    usersActionStartedAt.value = null
  }
}

async function toggleDisable(user: User) {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  usersToggleLoading.value = true
  usersActionStartedAt.value = Date.now()
  const signal = usersActionAbort.begin()
  try {
    // 供重试定位用户
    // context set on failure via reportAndNotify
    await apiPut(`/api/v1/users/${encodeURIComponent(user.name)}`, { ...user, disabled: !user.disabled }, { signal })
    await loadUsers()
    const okMsg = user.disabled ? t.value('usersEnabled') : t.value('usersDisabled')
    setActionOk(okMsg)
    success(okMsg)
  } catch (e) {
    reportUsersCatch('toggle', e, { name: user.name })
  } finally {
    usersActionAbort.end()
    usersToggleLoading.value = false
    usersActionStartedAt.value = null
  }
}

async function selectUser(user: User) {
  selectedUser.value = user
  try {
    const data = await apiGet<PermissionsResponse>(`/api/v1/users/${encodeURIComponent(user.name)}/database-permissions`)
    userGrants.value = data.grants ?? []
  } catch (e) {
    reportAndNotify('select-grants', e, { name: user.name })
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
  if (usersWriteLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  if (!grantDbs.value.length || !grantPerms.value.length || !selectedUser.value) return
  clearActionResult()
  usersWriteLoading.value = true
  usersActionStartedAt.value = Date.now()
  const signal = usersActionAbort.begin()
  try {
    const tasks: Promise<unknown>[] = []
    for (const db of grantDbs.value) {
      for (const perm of grantPerms.value) {
        tasks.push(apiPut(
          `/api/v1/users/${encodeURIComponent(selectedUser.value.name)}/database-permissions/${encodeURIComponent(db)}/${perm}`,
          undefined,
          { signal },
        ))
      }
    }
    await Promise.all(tasks)
    grantDbs.value = []
    grantPerms.value = []
    await selectUser(selectedUser.value)
    setActionOk(t.value('usersGrantOk'))
    success(t.value('usersGrantOk'))
  } catch (e) {
    reportUsersCatch('grant', e)
  } finally {
    usersActionAbort.end()
    usersWriteLoading.value = false
    usersActionStartedAt.value = null
  }
}

async function revokeGrant(g: DatabaseGrant) {
  if (usersWriteLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  if (!selectedUser.value) return
  usersWriteLoading.value = true
  usersActionStartedAt.value = Date.now()
  const signal = usersActionAbort.begin()
  try {
    await apiDelete(
      `/api/v1/users/${encodeURIComponent(selectedUser.value.name)}/database-permissions/${encodeURIComponent(g.database)}/${encodeURIComponent(g.permission)}`,
      { signal },
    )
    await selectUser(selectedUser.value)
    setActionOk(t.value('usersRevokeOk'))
    success(t.value('usersRevokeOk'))
  } catch (e) {
    reportUsersCatch('revoke', e, { database: g.database, permission: g.permission })
  } finally {
    usersActionAbort.end()
    usersWriteLoading.value = false
    usersActionStartedAt.value = null
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
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
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
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

function openBatch(mode: 'enable' | 'disable') {
  if (batchLoading.value) return
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineAdminBlocked')))
    return
  }
  if (!selectedIds.value.length) return
  batchMode.value = mode
  batchOpen.value = true
}

async function confirmBatch() {
  if (batchLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  const names = selectedIds.value.slice()
  if (!names.length) {
    batchOpen.value = false
    return
  }
  batchLoading.value = true
  batchProgress.value = { ...emptyBatchProgress(), total: names.length }
  usersActionStartedAt.value = Date.now()
  const signal = usersActionAbort.begin()
  const wantDisabled = batchMode.value === 'disable'
  let data: BatchMutationSummary | null = null
  const cancelledBox: { summary: BatchMutationSummary | null } = { summary: null }
  try {
    await apiPostNDJSONStream(
      '/api/v1/users/batch-disabled?stream=1',
      { names, disabled: wantDisabled },
      (_line, record, parseError) => {
        if (parseError || record == null) return
        const applied = applyBatchProgressEvent(batchProgress.value, record)
        batchProgress.value = applied.next
        if (applied.summary) {
          data = applied.summary
          if (applied.summary.cancelled) cancelledBox.summary = applied.summary
        }
        if (applied.error) throw new Error(applied.error)
      },
      { signal, headers: { Accept: 'application/x-ndjson' } },
    )
    const cancelledSummary = cancelledBox.summary
    if (cancelledSummary) {
      const processed = cancelledSummary.ok_count + cancelledSummary.skip_count + cancelledSummary.fail_count
      const msg = formatMessage(t.value('listBatchCancelledPartial'), {
        done: processed || batchProgress.value.done,
        total: batchProgress.value.total || processed,
        ok: cancelledSummary.ok_count,
        skip: cancelledSummary.skip_count,
        fail: cancelledSummary.fail_count,
      })
      setActionResult(makeActionResult('info', msg))
      info(msg)
      try { await loadUsers() } catch { /* soft */ }
      clearSelection()

      batchOpen.value = false
      return
    }
    if (!data) {
      data = {
        ok: batchProgress.value.fail === 0,
        ok_count: batchProgress.value.ok,
        skip_count: batchProgress.value.skip,
        fail_count: batchProgress.value.fail,
        items: [],
      }
    }
    await loadUsers()
    const ok = data.ok_count ?? 0
    const skip = data.skip_count ?? 0
    const fail = data.fail_count ?? 0
    const failNames = (data.items ?? [])
      .filter((it) => it.status === 'error')
      .map((it) => it.name)
    const key = fail ? 'listBatchPartial' : 'listBatchDone'
    let msg = formatMessage(t.value(key), { ok, skip, fail })
    if (failNames.length) {
      const shown = failNames.slice(0, 8).join(', ')
      const more = failNames.length > 8 ? `…(+${failNames.length - 8})` : ''
      msg = `${msg}；${formatMessage(t.value('listBatchFailDetail'), { names: shown + more })}`
    }
    if (fail) {
      lastFailedAction.value = 'batch'
      actionResult.value = makeActionResult('error', msg)
      notifyError(msg)
    } else {
      setActionOk(msg)
      success(msg)
    }
    clearSelection()
    batchOpen.value = false
  } catch (e) {
    if (isCanceledError(e) && batchProgress.value.done > 0) {
      const prog = batchProgress.value
      const msg = formatMessage(t.value('listBatchCancelledPartial'), {
        done: prog.done,
        total: prog.total || names.length,
        ok: prog.ok,
        skip: prog.skip,
        fail: prog.fail,
      })
      setActionResult(makeActionResult('info', msg))
      info(msg)
      try { await loadUsers() } catch { /* soft */ }
      clearSelection()
      batchOpen.value = false
    } else {
      reportUsersCatch('batch', e)
    }
  } finally {
    usersActionAbort.end()
    batchLoading.value = false
    usersActionStartedAt.value = null
    batchProgress.value = emptyBatchProgress()
  }
}
onBeforeUnmount(() => {
  cancelUsersAction()
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
        <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
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
      v-if="loadError && !users.length"
      kind="error"
      data-testid="users-load-error"
      :message="loadError"
      retryable
      @retry="loadUsers"
      @dismiss="loadError = ''"
    />
    <PartialErrorBanner
      v-else-if="loadError && users.length"
      :message="`${t('usersRefreshFailed')}：${loadError}`"
      test-id="users-refresh-error"
      @retry="loadUsers"
      @dismiss="loadError = ''"
    />
    <ActionResultBanner
      :result="actionResult"
      :retryable="canRetryAction"
      data-testid="users-action-result"
      @retry="retryLastUsersAction"
      @dismiss="clearActionResult"
    />
    <InFlightBanner
      :active="deleteLoading || batchLoading || usersToggleLoading || usersWriteLoading"
      :started-at-ms="usersActionStartedAt"
      kind="admin"
      :progress-percent="batchLoading ? batchProgressPercent(batchProgress) : null"
      :progress-label="batchLoading && batchProgress.total ? formatMessage(t('batchProgressLabel'), { done: batchProgress.done, total: batchProgress.total, ok: batchProgress.ok, skip: batchProgress.skip, fail: batchProgress.fail }) : undefined"
      @cancel="cancelUsersAction"
    />
    <PartialErrorBanner
      v-if="grantDbError"
      :message="`${t('usersGrantDbLoadFailed')}：${grantDbError}`"
      test-id="users-grant-db-error"
      @retry="loadGrantDatabases"
      @dismiss="grantDbError = ''"
    />

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
          <button type="button" class="mts-btn" data-testid="users-batch-enable" :disabled="!selectedCount || writeBlocked || batchLoading || usersWriteLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="openBatch('enable')">{{ t('listBatchEnable') }}</button>
          <button type="button" class="mts-btn" data-testid="users-batch-disable" :disabled="!selectedCount || writeBlocked || batchLoading || usersWriteLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="openBatch('disable')">{{ t('listBatchDisable') }}</button>
        </template>
      </ListSelectionToolbar>
    </div>

    <div v-if="!filteredUsers.length" class="mts-card">
      <EmptyState
        data-testid="users-empty-filter"
        :title="users.length ? t('usersFilterEmpty') : t('usersEmpty')"
        :description="users.length ? t('usersFilterEmptyDesc') : t('usersEmptyDesc')"
      >
        <template v-if="users.length" #action>
          <button type="button" class="mts-btn-primary" data-testid="users-clear-filters" @click="userFilter = ''; roleFilter = ''">{{ t('clearFilters') }}</button>
        </template>
        <template v-else #action>
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
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="users-sort-name" @click="cycleUserSort('name')" :aria-sort="ariaSortValue(userSort, 'name')">
              {{ t('usersColUser') }} <span aria-hidden="true">{{ userSortIndicator('name') }}</span>
            </button>
          </div>
          <div class="px-4 py-2.5">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="users-sort-role" @click="cycleUserSort('role')" :aria-sort="ariaSortValue(userSort, 'role')">
              {{ t('usersColRole') }} <span aria-hidden="true">{{ userSortIndicator('role') }}</span>
            </button>
          </div>
          <div class="px-4 py-2.5">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="users-sort-status" @click="cycleUserSort('status')" :aria-sort="ariaSortValue(userSort, 'status')">
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
      :loading="usersWriteLoading"
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
      allow-cancel-while-loading
      @confirm="confirmBatch"
      @cancel="cancelUsersAction"
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
      allow-cancel-while-loading
      @confirm="confirmDelete"
      @cancel="cancelUsersAction"
    />
  </div>
</template>
