<script setup lang="ts">
import { nextTick, onBeforeUnmount, watch } from 'vue'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'

const props = defineProps<{
  showCreate: boolean
  newUser: { name: string; display_name: string; password: string; role: string }
  showSetPassword: boolean
  setPasswordUser: string
  setPasswordValue: string
  showChangeSelfPassword: boolean
  selfOldPassword: string
  selfNewPassword: string
}>()

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
      <h3 id="create-user-title" class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">新建用户</h3>
      <div class="space-y-2">
        <input :value="newUser.name" @input="emit('update:newUser', { ...newUser, name: ($event.target as HTMLInputElement).value })" placeholder="用户名" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" />
        <input :value="newUser.display_name" @input="emit('update:newUser', { ...newUser, display_name: ($event.target as HTMLInputElement).value })" placeholder="显示名 (可选)" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" />
        <select :value="newUser.role" @change="emit('update:newUser', { ...newUser, role: ($event.target as HTMLSelectElement).value })" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800">
          <option value="user">普通用户</option>
          <option value="admin">管理员</option>
        </select>
        <input :value="newUser.password" @input="emit('update:newUser', { ...newUser, password: ($event.target as HTMLInputElement).value })" type="password" placeholder="密码 (可选)" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" />
      </div>
      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <button type="button" class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm dark:border-slate-700" @click="emit('update:showCreate', false)">取消</button>
        <button type="button" class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="emit('create-user')">创建</button>
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
      <h3 id="set-password-title" class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">设置密码 · {{ setPasswordUser }}</h3>
      <input :value="setPasswordValue" @input="emit('update:setPasswordValue', ($event.target as HTMLInputElement).value)" type="password" placeholder="输入新密码" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" @keyup.enter="emit('set-password')" />
      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <button type="button" class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm dark:border-slate-700" @click="emit('update:showSetPassword', false)">取消</button>
        <button type="button" class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="emit('set-password')">设置</button>
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
      <h3 id="change-password-title" class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">修改我的密码</h3>
      <div class="space-y-2">
        <input :value="selfOldPassword" @input="emit('update:selfOldPassword', ($event.target as HTMLInputElement).value)" type="password" placeholder="当前密码" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" />
        <input :value="selfNewPassword" @input="emit('update:selfNewPassword', ($event.target as HTMLInputElement).value)" type="password" placeholder="新密码" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" @keyup.enter="emit('change-password')" />
      </div>
      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <button type="button" class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm dark:border-slate-700" @click="emit('update:showChangeSelfPassword', false)">取消</button>
        <button type="button" class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="emit('change-password')">确认修改</button>
      </div>
    </div>
  </div>
</template>
