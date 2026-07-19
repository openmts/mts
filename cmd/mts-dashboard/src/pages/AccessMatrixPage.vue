<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAuth } from '@/composables/useAuth'
import {
  ACCESS_LEVEL_LABEL,
  RBAC_CAPABILITY_MATRIX,
  countByLevel,
  levelForRole,
  matrixAreas,
  type AccessLevel,
  type RoleName,
} from '@/utils/rbacMatrix'
import { Shield } from 'lucide-vue-next'

const { currentRole } = useAuth()
const roleFilter = ref<'all' | RoleName>('all')
const areaFilter = ref('')
const areas = matrixAreas()

const displayRole = computed(() => (currentRole.value === 'admin' ? 'admin' : 'user') as RoleName)

const rows = computed(() => {
  return RBAC_CAPABILITY_MATRIX.filter((r) => {
    if (areaFilter.value && r.area !== areaFilter.value) return false
    if (roleFilter.value === 'all') return true
    return levelForRole(r, roleFilter.value) !== 'none'
  })
})

const adminCounts = countByLevel('admin')
const userCounts = countByLevel('user')

function levelClass(level: AccessLevel): string {
  switch (level) {
    case 'full':
      return 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-200'
    case 'self':
      return 'bg-sky-50 text-sky-800 dark:bg-sky-950/50 dark:text-sky-200'
    case 'data_scoped':
      return 'bg-amber-50 text-amber-900 dark:bg-amber-950/40 dark:text-amber-100'
    default:
      return 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400'
  }
}

function levelLabel(level: AccessLevel): string {
  return ACCESS_LEVEL_LABEL[level].zh
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <Shield class="h-5 w-5" />
          权限能力矩阵
        </h1>
        <p class="text-xs mts-muted">
          对照控制台页面与后端角色语义。当前会话角色：
          <span class="font-medium text-slate-800 dark:text-slate-100">{{ displayRole }}</span>
        </p>
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="mts-card p-3">
        <p class="text-xs font-medium text-slate-700 dark:text-slate-200">admin 分布</p>
        <p class="mt-1 text-xs mts-muted">
          全部 {{ adminCounts.full }} · 自身 {{ adminCounts.self }} · 库级 {{ adminCounts.data_scoped }} · 无 {{ adminCounts.none }}
        </p>
      </div>
      <div class="mts-card p-3">
        <p class="text-xs font-medium text-slate-700 dark:text-slate-200">user 分布</p>
        <p class="mt-1 text-xs mts-muted">
          全部 {{ userCounts.full }} · 自身 {{ userCounts.self }} · 库级 {{ userCounts.data_scoped }} · 无 {{ userCounts.none }}
        </p>
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <label class="text-xs mts-muted">角色过滤</label>
      <select v-model="roleFilter" class="mts-input w-auto text-sm">
        <option value="all">全部能力行</option>
        <option value="admin">仅 admin 可用</option>
        <option value="user">仅 user 可用</option>
      </select>
      <label class="text-xs mts-muted ml-2">区域</label>
      <select v-model="areaFilter" class="mts-input w-auto text-sm">
        <option value="">全部区域</option>
        <option v-for="a in areas" :key="a" :value="a">{{ a }}</option>
      </select>
    </div>

    <div class="mts-card overflow-auto">
      <table class="min-w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wide text-slate-500 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-400">
          <tr>
            <th class="px-3 py-2 font-medium">区域</th>
            <th class="px-3 py-2 font-medium">能力</th>
            <th class="px-3 py-2 font-medium">admin</th>
            <th class="px-3 py-2 font-medium">user</th>
            <th class="px-3 py-2 font-medium">路由</th>
            <th class="px-3 py-2 font-medium">备注</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in rows"
            :key="row.id"
            class="border-b border-slate-100 dark:border-slate-800"
          >
            <td class="px-3 py-2 whitespace-nowrap text-slate-600 dark:text-slate-300">{{ row.area }}</td>
            <td class="px-3 py-2 text-slate-800 dark:text-slate-100">{{ row.capability }}</td>
            <td class="px-3 py-2">
              <span class="inline-flex rounded-full px-2 py-0.5 text-xs" :class="levelClass(row.admin)">
                {{ levelLabel(row.admin) }}
              </span>
            </td>
            <td class="px-3 py-2">
              <span class="inline-flex rounded-full px-2 py-0.5 text-xs" :class="levelClass(row.user)">
                {{ levelLabel(row.user) }}
              </span>
            </td>
            <td class="px-3 py-2 font-mono text-xs text-slate-500 dark:text-slate-400">{{ row.route || '—' }}</td>
            <td class="px-3 py-2 text-xs mts-muted">{{ row.notes || '—' }}</td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="6" class="px-3 py-8 text-center text-sm mts-muted">无匹配能力</td>
          </tr>
        </tbody>
      </table>
    </div>

    <p class="text-xs mts-muted">
      说明：本矩阵为产品语义对照表，最终以服务端鉴权为准。库级授权通过
      <code class="font-mono">/api/v1/users/.../database-permissions</code>
      与
      <code class="font-mono">/api/v1/authz/database/check</code>
      落实。管理员可打开
      <RouterLink class="underline" to="/access/grants">实时授权总览</RouterLink>
      复核当前 grants。
    </p>
  </div>
</template>
