<script setup lang="ts">
import { Shield, X } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { permissionLabel, DB_PERMISSIONS } from '@/utils/permissionLabel'
import { computed, ref } from 'vue'
import EmptyState from '@/components/EmptyState.vue'
import { filterDatabaseNames, sortGrants } from '@/utils/grantPanelView'

export interface GrantItem { database: string; permission: string }
export interface UserItem { name: string; display_name?: string; role?: string; disabled?: boolean }

const props = withDefaults(
  defineProps<{
    offline?: boolean
    writeBlocked?: boolean
    blockReason?: 'none' | 'offline' | 'session' | null
    selectedUser: UserItem
    userGrants: GrantItem[]
    databases: string[]
    grantDbs: string[]
    grantPerms: string[]
  }>(),
  { offline: false, writeBlocked: undefined, blockReason: null },
)

const emit = defineEmits<{
  close: []
  'toggle-db': [string]
  'toggle-perm': [string]
  grant: []
  revoke: [GrantItem]
}>()

const { t, locale } = useI18n()

const blocked = () => props.writeBlocked ?? props.offline
function blockedTitle(fallback?: string): string | undefined {
  if (!blocked()) return fallback
  if (props.blockReason === 'session') return t.value('sessionMutationBlocked')
  return t.value('offlineAdminBlocked')
}

const uiLocale = computed(() => (locale.value === 'en' ? 'en' : 'zh') as 'en' | 'zh')
function permText(p: string) { return permissionLabel(p, uiLocale.value) }

const dbFilter = ref('')
const filteredDatabases = computed(() => filterDatabaseNames(props.databases, dbFilter.value))
const sortedGrants = computed(() => sortGrants(props.userGrants))
const canGrant = computed(() => props.grantDbs.length > 0 && props.grantPerms.length > 0)
</script>

<template>
  <div id="user-grant-panel" class="scroll-mt-20 rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-700 dark:bg-slate-900" data-testid="user-grant-panel">
    <div class="mb-3 flex items-center justify-between">
      <h3 class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
        <Shield class="h-4 w-4 text-slate-500 dark:text-slate-400" />
        {{ formatMessage(t('grantPanelTitle'), { name: selectedUser.name }) }}
      </h3>
      <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300" data-testid="user-grant-close" @click="emit('close')"><X class="h-4 w-4" /></button>
    </div>
    <div class="mb-3">
      <div class="mb-1 flex items-center justify-between gap-2">
        <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('grantCurrent') }}</p>
        <span class="text-[11px] mts-muted" data-testid="user-grant-count">{{ sortedGrants.length }}</span>
      </div>
      <EmptyState
        v-if="!sortedGrants.length"
        compact
        data-testid="user-grant-empty"
        :title="t('grantEmpty')"
        :description="t('grantEmptyDesc')"
      />
      <div v-else class="flex flex-wrap gap-1.5" data-testid="user-grant-chips">
        <button
          v-for="g in sortedGrants"
          :key="g.database + g.permission"
          type="button"
          class="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-700 hover:bg-red-50 hover:text-red-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:text-red-200 disabled:cursor-not-allowed disabled:opacity-40"
          :title="blockedTitle(t('grantRevokeTitle'))"
          :disabled="blocked()"
          :data-testid="`user-grant-revoke-${g.database}-${g.permission}`"
          @click="emit('revoke', g)"
        >{{ g.database }}:{{ permText(g.permission) }}</button>
      </div>
    </div>
    <div class="grid gap-3 sm:grid-cols-2">
      <div>
        <div class="mb-1 flex items-center justify-between gap-2">
          <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('database') }}</p>
          <span class="text-[11px] mts-muted" data-testid="user-grant-db-filter-count">{{ filteredDatabases.length }}/{{ databases.length }}</span>
        </div>
        <input
          v-model="dbFilter"
          class="mts-input mb-1.5 text-xs"
          data-testid="user-grant-db-filter"
          :placeholder="t('grantDbFilterPh')"
        />
        <div class="max-h-28 space-y-1 overflow-auto rounded border border-slate-100 p-2 dark:border-slate-800" data-testid="user-grant-db-list">
          <EmptyState
            v-if="!filteredDatabases.length"
            compact
            data-testid="user-grant-db-empty"
            :title="t('grantDbFilterEmpty')"
            :description="t('grantDbFilterEmptyDesc')"
          >
            <template v-if="databases.length" #action>
              <button type="button" class="mts-btn-primary" data-testid="user-grant-db-clear-filters" @click="dbFilter = ''">{{ t('clearFilters') }}</button>
            </template>
          </EmptyState>
          <label v-for="db in filteredDatabases" :key="db" class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300">
            <input type="checkbox" :checked="grantDbs.includes(db)" :data-testid="`user-grant-db-${db}`" @change="emit('toggle-db', db)" />
            {{ db }}
          </label>
        </div>
      </div>
      <div>
        <p class="mb-1 text-xs text-slate-500 dark:text-slate-400">{{ t('accessGrantsPermission') }}</p>
        <div class="space-y-1 rounded border border-slate-100 p-2 dark:border-slate-800" data-testid="user-grant-perm-list">
          <label v-for="perm in DB_PERMISSIONS" :key="perm" class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300">
            <input type="checkbox" :checked="grantPerms.includes(perm)" :data-testid="`user-grant-perm-${perm}`" @change="emit('toggle-perm', perm)" />
            {{ permText(perm) }}
          </label>
        </div>
      </div>
    </div>
    <div class="mt-3 flex flex-wrap items-center justify-between gap-2">
      <p class="text-[11px] mts-muted" data-testid="user-grant-selection-hint">
        {{ formatMessage(t('grantSelectionHint'), { dbs: grantDbs.length, perms: grantPerms.length }) }}
      </p>
      <button
        type="button"
        class="rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700 disabled:opacity-50"
        data-testid="user-grant-submit"
        :disabled="!canGrant || blocked()"
        :title="blockedTitle()"
        @click="emit('grant')"
      >{{ t('grantAction') }}</button>
    </div>
  </div>
</template>
