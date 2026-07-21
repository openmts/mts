<script setup lang="ts">
import { nextTick, onBeforeUnmount, watch } from 'vue'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'

const props = withDefaults(
  defineProps<{
    /** @deprecated 兼容旧用法；优先 writeBlocked */
    offline?: boolean
    writeBlocked?: boolean
    blockReason?: 'none' | 'offline' | 'session' | null
    showCreate: boolean
    newUser: { name: string; display_name: string; password: string; role: string }
    showSetPassword: boolean
    setPasswordUser: string
    setPasswordValue: string
    showChangeSelfPassword: boolean
    selfOldPassword: string
    selfNewPassword: string
  }>(),
  { offline: false, writeBlocked: undefined, blockReason: null },
)

const emit = defineEmits<{
  'update:showCreate': [boolean]
  'update:newUser': [typeof props.newUser]
  'update:showSetPassword': [boolean]
  'update:setPasswordUser': [string]
  'update:setPasswordValue': [string]
  'update:showChangeSelfPassword': [boolean]
  'update:selfOldPassword': [string]
  'update:selfNewPassword': [string]
  'create-user': []
  'set-password': []
  'change-password': []
}>()

const { t } = useI18n()

const blocked = () => props.writeBlocked ?? props.offline
function blockedTitle(): string | undefined {
  if (!blocked()) return undefined
  if (props.blockReason === 'session') return t.value('sessionMutationBlocked')
  return t.value('offlineAdminBlocked')
}

let trap: FocusTrapHandle | null = null

function closeAll() {
  if (props.showCreate) emit('update:showCreate', false)
  if (props.showSetPassword) emit('update:showSetPassword', false)
  if (props.showChangeSelfPassword) emit('update:showChangeSelfPassword', false)
}

function releaseTrap() {
  trap?.release()
  trap = null
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault()
    closeAll()
  }
}

async function attachTrap(sel: string) {
  releaseTrap()
  await nextTick()
  const root = document.querySelector(sel) as HTMLElement | null
  const panel = root?.querySelector('[data-dialog-panel]') as HTMLElement | null
  if (!panel) return
  trap = createFocusTrap(panel)
  trap.focusFirst()
}

watch(
  () => [props.showCreate, props.showSetPassword, props.showChangeSelfPassword] as const,
  async ([c, s, p]) => {
    const open = c || s || p
    window.removeEventListener('keydown', onKey)
    document.body.style.overflow = open ? 'hidden' : ''
    if (!open) {
      releaseTrap()
      return
    }
    window.addEventListener('keydown', onKey)
    if (c) await attachTrap('[data-modal="create-user"]')
    else if (s) await attachTrap('[data-modal="set-password"]')
    else await attachTrap('[data-modal="change-password"]')
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  releaseTrap()
  document.body.style.overflow = ''
})
</script>

<template>
  <div
    v-if="showCreate"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-3 sm:p-4"
    role="presentation"
    data-modal="create-user"
    @click.self="emit('update:showCreate', false)"
  >
    <div
      class="w-full max-w-sm rounded-xl bg-white p-4 shadow-xl dark:bg-slate-900 sm:p-5"
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-user-title"
      data-dialog-panel
    >
      <h3 id="create-user-title" class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('usersCreateTitle') }}</h3>
      <div class="space-y-2">
        <input data-testid="users-create-name" :value="newUser.name" @input="emit('update:newUser', { ...newUser, name: ($event.target as HTMLInputElement).value })" :placeholder="t('username')" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" />
        <input data-testid="users-create-display" :value="newUser.display_name" @input="emit('update:newUser', { ...newUser, display_name: ($event.target as HTMLInputElement).value })" :placeholder="t('displayNameOptional')" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" />
        <select data-testid="users-create-role" :value="newUser.role" @change="emit('update:newUser', { ...newUser, role: ($event.target as HTMLSelectElement).value })" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800">
          <option value="user">{{ t('roleUser') }}</option>
          <option value="admin">{{ t('roleAdmin') }}</option>
        </select>
        <input data-testid="users-create-password" :value="newUser.password" @input="emit('update:newUser', { ...newUser, password: ($event.target as HTMLInputElement).value })" type="password" :placeholder="t('passwordOptional')" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" />
      </div>
      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <button type="button" class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm dark:border-slate-700" data-testid="users-create-cancel" @click="emit('update:showCreate', false)">{{ t('cancel') }}</button>
        <button type="button" class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-50" data-testid="users-create-submit" :disabled="blocked()" :title="blockedTitle()" @click="emit('create-user')">{{ t('create') }}</button>
      </div>
    </div>
  </div>

  <div
    v-if="showSetPassword"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-3 sm:p-4"
    role="presentation"
    data-modal="set-password"
    @click.self="emit('update:showSetPassword', false)"
  >
    <div
      class="w-full max-w-sm rounded-xl bg-white p-4 shadow-xl dark:bg-slate-900 sm:p-5"
      role="dialog"
      aria-modal="true"
      aria-labelledby="set-password-title"
      data-dialog-panel
    >
      <h3 id="set-password-title" class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ formatMessage(t('usersSetPasswordTitle'), { name: setPasswordUser }) }}</h3>
      <input :value="setPasswordValue" @input="emit('update:setPasswordValue', ($event.target as HTMLInputElement).value)" type="password" :placeholder="t('usersNewPasswordPlaceholder')" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" @keyup.enter="emit('set-password')" />
      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <button type="button" class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm dark:border-slate-700" @click="emit('update:showSetPassword', false)">{{ t('cancel') }}</button>
        <button type="button" class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-50" data-testid="users-set-password-submit" :disabled="blocked()" :title="blockedTitle()" @click="emit('set-password')">{{ t('usersSetAction') }}</button>
      </div>
    </div>
  </div>

  <div
    v-if="showChangeSelfPassword"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-3 sm:p-4"
    role="presentation"
    data-modal="change-password"
    @click.self="emit('update:showChangeSelfPassword', false)"
  >
    <div
      class="w-full max-w-sm rounded-xl bg-white p-4 shadow-xl dark:bg-slate-900 sm:p-5"
      role="dialog"
      aria-modal="true"
      aria-labelledby="change-password-title"
      data-dialog-panel
    >
      <h3 id="change-password-title" class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('usersChangeMyPassword') }}</h3>
      <div class="space-y-2">
        <input :value="selfOldPassword" @input="emit('update:selfOldPassword', ($event.target as HTMLInputElement).value)" type="password" :placeholder="t('accountOldPassword')" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" />
        <input :value="selfNewPassword" @input="emit('update:selfNewPassword', ($event.target as HTMLInputElement).value)" type="password" :placeholder="t('accountNewPassword')" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" @keyup.enter="emit('change-password')" />
      </div>
      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <button type="button" class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm dark:border-slate-700" @click="emit('update:showChangeSelfPassword', false)">{{ t('cancel') }}</button>
        <button type="button" class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-50" data-testid="users-change-self-submit" :disabled="blocked()" :title="blockedTitle()" @click="emit('change-password')">{{ t('usersConfirmChange') }}</button>
      </div>
    </div>
  </div>
</template>
