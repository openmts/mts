<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import {
  filterGrantRows,
  flattenUserGrants,
  grantCoverage,
  type GrantRow,
  type UserGrantBundle,
} from '@/utils/grantsSummary'
import { RefreshCw, ShieldCheck } from 'lucide-vue-next'

interface User {
  name: string
  role?: string
  disabled?: boolean
}
interface UsersResponse { users: User[] }
interface PermissionsResponse { grants: Array<{ database: string; permission: string }> }

const { isAdmin } = useAuth()
const loading = ref(false)
const loadError = ref('')
const rows = ref<GrantRow[]>([])
const userFilter = ref('')
const dbFilter = ref('')
const permFilter = ref('')
const q = ref('')
const partialErrors = ref<string[]>([])

const users = computed(() => Array.from(new Set(rows.value.map((r) => r.user))).sort())
const databases = computed(() => Array.from(new Set(rows.value.map((r) => r.database))).sort())
const permissions = computed(() => Array.from(new Set(rows.value.map((r) => r.permission))).sort())

const filtered = computed(() =>
  filterGrantRows(rows.value, {
    user: userFilter.value,
    database: dbFilter.value,
    permission: permFilter.value,
    q: q.value,
  }),
)
const coverage = computed(() => grantCoverage(filtered.value))

async function load() {
  if (!isAdmin.value) return
  loading.value = true
  loadError.value = ''
  partialErrors.value = []
  try {
    const list = await apiGet<UsersResponse>('/api/v1/users')
    const usersList = list.users ?? []
    const bundles: UserGrantBundle[] = []
    const errs: string[] = []
    // 并发拉取，但限制并发数避免压垮 POC 单机
    const concurrency = 4
    let idx = 0
    async function worker() {
      while (idx < usersList.length) {
        const i = idx++
        const u = usersList[i]
        try {
          const data = await apiGet<PermissionsResponse>(
            `/api/v1/users/${encodeURIComponent(u.name)}/database-permissions`,
          )
          bundles.push({
            user: u.name,
            role: u.role,
            disabled: u.disabled,
            grants: data.grants ?? [],
          })
        } catch (e) {
          errs.push(`${u.name}: ${e instanceof Error ? e.message : 'load grants failed'}`)
          bundles.push({ user: u.name, role: u.role, disabled: u.disabled, grants: [] })
        }
      }
    }
    await Promise.all(Array.from({ length: Math.min(concurrency, usersList.length || 1) }, () => worker()))
    rows.value = flattenUserGrants(bundles)
    partialErrors.value = errs
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载授权失败'
    rows.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => { void load() })
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <ShieldCheck class="h-5 w-5" />
          实时授权总览
        </h1>
        <p class="text-xs mts-muted">
          拉取全部用户的库级 grants（/users/.../database-permissions），用于上线前权限复核。
        </p>
      </div>
      <button class="mts-btn" :disabled="loading" @click="load">
        <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
        刷新
      </button>
    </div>

    <ActionResultBanner v-if="loadError" kind="error" :message="loadError" @dismiss="loadError = ''" />
    <ActionResultBanner
      v-else-if="partialErrors.length"
      kind="warn"
      :message="`部分用户授权拉取失败：${partialErrors.slice(0, 3).join('；')}${partialErrors.length > 3 ? '…' : ''}`"
      :dismissible="false"
    />

    <div class="grid gap-3 sm:grid-cols-3">
      <div class="mts-card p-3">
        <p class="text-xs mts-muted">用户（过滤后）</p>
        <p class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ coverage.users }}</p>
      </div>
      <div class="mts-card p-3">
        <p class="text-xs mts-muted">数据库</p>
        <p class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ coverage.databases }}</p>
      </div>
      <div class="mts-card p-3">
        <p class="text-xs mts-muted">授权条数</p>
        <p class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ coverage.grants }}</p>
      </div>
    </div>

    <div class="flex flex-wrap items-end gap-2">
      <label class="text-xs mts-muted">用户
        <select v-model="userFilter" class="mts-input mt-1 w-auto min-w-[8rem] text-sm">
          <option value="">全部</option>
          <option v-for="u in users" :key="u" :value="u">{{ u }}</option>
        </select>
      </label>
      <label class="text-xs mts-muted">数据库
        <select v-model="dbFilter" class="mts-input mt-1 w-auto min-w-[8rem] text-sm">
          <option value="">全部</option>
          <option v-for="d in databases" :key="d" :value="d">{{ d }}</option>
        </select>
      </label>
      <label class="text-xs mts-muted">权限
        <select v-model="permFilter" class="mts-input mt-1 w-auto min-w-[8rem] text-sm">
          <option value="">全部</option>
          <option v-for="p in permissions" :key="p" :value="p">{{ p }}</option>
        </select>
      </label>
      <label class="text-xs mts-muted grow">搜索
        <input v-model="q" class="mts-input mt-1 text-sm" placeholder="user / db / permission" />
      </label>
    </div>

    <div v-if="!loading && !filtered.length" class="mts-card">
      <EmptyState
        title="暂无授权记录"
        description="可在用户页为非 admin 账号授予库级 read/write，或调整过滤条件。"
      />
    </div>
    <div v-else class="mts-card overflow-auto">
      <table class="min-w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wide text-slate-500 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-400">
          <tr>
            <th class="px-3 py-2 font-medium">用户</th>
            <th class="px-3 py-2 font-medium">角色</th>
            <th class="px-3 py-2 font-medium">状态</th>
            <th class="px-3 py-2 font-medium">数据库</th>
            <th class="px-3 py-2 font-medium">权限</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(row, i) in filtered"
            :key="`${row.user}-${row.database}-${row.permission}-${i}`"
            class="border-b border-slate-100 dark:border-slate-800"
          >
            <td class="px-3 py-2 font-medium text-slate-800 dark:text-slate-100">{{ row.user }}</td>
            <td class="px-3 py-2 text-slate-600 dark:text-slate-300">{{ row.role || '—' }}</td>
            <td class="px-3 py-2">
              <span
                class="inline-flex rounded-full px-2 py-0.5 text-xs"
                :class="row.disabled
                  ? 'bg-rose-50 text-rose-800 dark:bg-rose-950/40 dark:text-rose-100'
                  : 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-100'"
              >
                {{ row.disabled ? '禁用' : '启用' }}
              </span>
            </td>
            <td class="px-3 py-2 font-mono text-xs">{{ row.database }}</td>
            <td class="px-3 py-2">
              <span class="inline-flex rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-700 dark:bg-slate-800 dark:text-slate-200">
                {{ row.permission }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
