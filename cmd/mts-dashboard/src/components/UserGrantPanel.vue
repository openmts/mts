<script setup lang="ts">
import { Shield, X } from 'lucide-vue-next'

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
</script>

<template>
  <div class="rounded-xl border border-slate-200 bg-white p-5">
    <div class="mb-3 flex items-center justify-between">
      <h3 class="flex items-center gap-2 text-sm font-semibold text-slate-800">
        <Shield class="h-4 w-4 text-slate-500" />
        权限 · {{ selectedUser.name }}
      </h3>
      <button class="rounded p-1 text-slate-400 hover:text-slate-600" @click="emit('close')"><X class="h-4 w-4" /></button>
    </div>
    <div class="mb-3">
      <p class="mb-1 text-xs text-slate-500">当前授权</p>
      <div v-if="!userGrants.length" class="text-xs text-slate-400">无库权限</div>
      <div v-else class="flex flex-wrap gap-1.5">
        <button
          v-for="g in userGrants"
          :key="g.database + g.permission"
          class="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-700 hover:bg-red-50 hover:text-red-700"
          title="点击撤销"
          @click="emit('revoke', g)"
        >{{ g.database }}:{{ g.permission }}</button>
      </div>
    </div>
    <div class="grid gap-3 sm:grid-cols-2">
      <div>
        <p class="mb-1 text-xs text-slate-500">数据库</p>
        <div class="max-h-28 space-y-1 overflow-auto rounded border border-slate-100 p-2">
          <label v-for="db in databases" :key="db" class="flex items-center gap-2 text-xs text-slate-600">
            <input type="checkbox" :checked="grantDbs.includes(db)" @change="emit('toggle-db', db)" />
            {{ db }}
          </label>
        </div>
      </div>
      <div>
        <p class="mb-1 text-xs text-slate-500">权限</p>
        <div class="space-y-1 rounded border border-slate-100 p-2">
          <label v-for="perm in ['read', 'write', 'admin']" :key="perm" class="flex items-center gap-2 text-xs text-slate-600">
            <input type="checkbox" :checked="grantPerms.includes(perm)" @change="emit('toggle-perm', perm)" />
            {{ perm }}
          </label>
        </div>
      </div>
    </div>
    <div class="mt-3 flex justify-end">
      <button class="rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700" @click="emit('grant')">授权</button>
    </div>
  </div>
</template>
