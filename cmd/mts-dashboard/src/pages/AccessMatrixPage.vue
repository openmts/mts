<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAuth } from '@/composables/useAuth'
import {
  ACCESS_LEVEL_LABEL,
  RBAC_CAPABILITY_MATRIX,
  countByLevel,
  levelForRole,
  matrixAreas,
  textForLocale,
  type AccessLevel,
  type LocaleCode,
  type RoleName,
} from '@/utils/rbacMatrix'
import { Shield } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'

const { currentRole } = useAuth()
const { t, locale } = useI18n()
const roleFilter = ref<'all' | RoleName>('all')
const areaFilter = ref('')
const uiLocale = computed<LocaleCode>(() => (locale.value === 'en' ? 'en' : 'zh'))
const areas = computed(() =>
  matrixAreas().map((a) => ({ key: a.key, label: textForLocale(a.label, uiLocale.value) })),
)

const displayRole = computed(() => (currentRole.value === 'admin' ? 'admin' : 'user') as RoleName)

const rows = computed(() => {
  return RBAC_CAPABILITY_MATRIX.filter((r) => {
    if (areaFilter.value && r.areaKey !== areaFilter.value) return false
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
  return textForLocale(ACCESS_LEVEL_LABEL[level], uiLocale.value)
}
function roleLabel(role: string): string {
  if (role === 'admin') return t.value('roleAdmin')
  if (role === 'user') return t.value('roleUser')
  return role
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <Shield class="h-5 w-5" />
          {{ t('accessMatrixTitle') }}
        </h1>
        <p class="text-xs mts-muted">
          {{ t('accessMatrixDesc') }}
          <span class="font-medium text-slate-800 dark:text-slate-100">{{ roleLabel(displayRole) }}</span>
        </p>
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="mts-card p-3">
        <p class="text-xs font-medium text-slate-700 dark:text-slate-200">{{ formatMessage(t('accessMatrixAdminDist'), { role: t('roleAdmin') }) }}</p>
        <p class="mt-1 text-xs mts-muted">
          {{ formatMessage(t('accessMatrixDistLine'), { full: adminCounts.full, self: adminCounts.self, data: adminCounts.data_scoped, none: adminCounts.none }) }}
        </p>
      </div>
      <div class="mts-card p-3">
        <p class="text-xs font-medium text-slate-700 dark:text-slate-200">{{ formatMessage(t('accessMatrixUserDist'), { role: t('roleUser') }) }}</p>
        <p class="mt-1 text-xs mts-muted">
          {{ formatMessage(t('accessMatrixDistLine'), { full: userCounts.full, self: userCounts.self, data: userCounts.data_scoped, none: userCounts.none }) }}
        </p>
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <label class="text-xs mts-muted">{{ t('accessMatrixRoleFilter') }}</label>
      <select v-model="roleFilter" class="mts-input w-auto text-sm">
        <option value="all">{{ t('accessMatrixAllRows') }}</option>
        <option value="admin">{{ formatMessage(t('accessMatrixAdminOnly'), { role: t('roleAdmin') }) }}</option>
        <option value="user">{{ formatMessage(t('accessMatrixUserOnly'), { role: t('roleUser') }) }}</option>
      </select>
      <label class="text-xs mts-muted ml-2">{{ t('accessMatrixArea') }}</label>
      <select v-model="areaFilter" class="mts-input w-auto text-sm">
        <option value="">{{ t('accessMatrixAllAreas') }}</option>
        <option v-for="a in areas" :key="a.key" :value="a.key">{{ a.label }}</option>
      </select>
    </div>

    <div class="mts-card overflow-auto">
      <table class="min-w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wide text-slate-500 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-400">
          <tr>
            <th class="px-3 py-2 font-medium">{{ t('accessMatrixColArea') }}</th>
            <th class="px-3 py-2 font-medium">{{ t('accessMatrixColCapability') }}</th>
            <th class="px-3 py-2 font-medium">{{ t('roleAdmin') }}</th>
            <th class="px-3 py-2 font-medium">{{ t('roleUser') }}</th>
            <th class="px-3 py-2 font-medium">{{ t('accessMatrixColRoute') }}</th>
            <th class="px-3 py-2 font-medium">{{ t('accessMatrixColNote') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in rows"
            :key="row.id"
            class="border-b border-slate-100 dark:border-slate-800"
          >
            <td class="px-3 py-2 whitespace-nowrap text-slate-600 dark:text-slate-300">{{ textForLocale(row.area, uiLocale) }}</td>
            <td class="px-3 py-2 text-slate-800 dark:text-slate-100">{{ textForLocale(row.capability, uiLocale) }}</td>
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
            <td class="px-3 py-2 font-mono text-xs text-slate-500 dark:text-slate-400">{{ row.route || t('emptyValue') }}</td>
            <td class="px-3 py-2 text-xs mts-muted">{{ textForLocale(row.notes, uiLocale) || t('emptyValue') }}</td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="6" class="px-3 py-8 text-center text-sm mts-muted">{{ t('accessMatrixEmpty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <p class="text-xs mts-muted">
      {{ t('accessMatrixFootnote') }} <RouterLink class="underline" to="/access/grants">{{ t('accessMatrixLiveGrantsLink') }}</RouterLink>。
    </p>
  </div>
</template>
