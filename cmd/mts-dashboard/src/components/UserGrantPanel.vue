<script setup lang="ts">
import { Shield, X } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { permissionLabel, DB_PERMISSIONS } from '@/utils/permissionLabel'
import { computed } from 'vue'

export interface GrantItem { database: string; permission: string }
export interface UserItem { name: string; display_name?: string; role?: string; disabled?: boolean }

defineProps<{
  selectedUser: UserItem
  userGrants: GrantItem[]
  databases: string[]
  grantDbs: string[]
  grantPerms: string[]
}>()

const emit = defineEmits<{
  close: []
  'toggle-db': [string]
  'toggle-perm': [string]
  grant: []
  revoke: [GrantItem]
}>()

const { t, locale } = useI18n()
const uiLocale = computed(() => (locale.value === 'en' ? 'en' : 'zh') as 'en' | 'zh')
function permText(p: string) { return permissionLabel(p, uiLocale.value) }
</script>

<template>
  <div class="rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900 p-5">
    <div class="mb-3 flex items-center justify-between">
      <h3 class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
        <Shield class="h-4 w-4 text-slate-500 dark:text-slate-400 dark:text-slate-500" />
        {{ formatMessage(t('grantPanelTitle'), { name: selectedUser.name }) }}
      </h3>
      <button class="rounded p-1 text-slate-400 dark:text-slate-500 hover:text-slate-600 dark:text-slate-300" @click="emit('close')"><X class="h-4 w-4" /></button>
    </div>
    <div class="mb-3">
      <p class="mb-1 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('grantCurrent') }}</p>
      <div v-if="!userGrants.length" class="text-xs text-slate-400 dark:text-slate-500">{{ t('grantEmpty') }}</div>
      <div v-else class="flex flex-wrap gap-1.5">
        <button
          v-for="g in userGrants"
          :key="g.database + g.permission"
          class="inline-flex items-center gap-1 rounded-full bg-slate-100 dark:bg-slate-800 px-2 py-0.5 text-xs text-slate-700 dark:text-slate-200 hover:bg-red-50 hover:text-red-700 dark:text-red-200"
          :title="t('grantRevokeTitle')"
          @click="emit('revoke', g)"
        >{{ g.database }}:{{ permText(g.permission) }}</button>
      </div>
    </div>
    <div class="grid gap-3 sm:grid-cols-2">
      <div>
        <p class="mb-1 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('database') }}</p>
        <div class="max-h-28 space-y-1 overflow-auto rounded border border-slate-100 p-2">
          <label v-for="db in databases" :key="db" class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300">
            <input type="checkbox" :checked="grantDbs.includes(db)" @change="emit('toggle-db', db)" />
            {{ db }}
          </label>
        </div>
      </div>
      <div>
        <p class="mb-1 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('accessGrantsPermission') }}</p>
        <div class="space-y-1 rounded border border-slate-100 p-2">
          <label v-for="perm in DB_PERMISSIONS" :key="perm" class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300">
            <input type="checkbox" :checked="grantPerms.includes(perm)" @change="emit('toggle-perm', perm)" />
            {{ permText(perm) }}
          </label>
        </div>
      </div>
    </div>
    <div class="mt-3 flex justify-end">
      <button class="rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700" @click="emit('grant')">{{ t('grantAction') }}</button>
    </div>
  </div>
</template>
