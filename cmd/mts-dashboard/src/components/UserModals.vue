<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, watch } from 'vue'

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

function anyOpen() {
  return props.showCreate || props.showSetPassword || props.showChangeSelfPassword
}

function closeAll() {
  if (props.showCreate) emit('update:showCreate', false)
  if (props.showSetPassword) emit('update:showSetPassword', false)
  if (props.showChangeSelfPassword) emit('update:showChangeSelfPassword', false)
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') closeAll()
}

function focusFirst(sel: string) {
  const el = document.querySelector(sel) as HTMLElement | null
  el?.focus()
}

watch(() => anyOpen(), async (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
  if (!open) return
  await nextTick()
  if (props.showCreate) focusFirst('[data-modal="create-user"] input')
  else if (props.showSetPassword) focusFirst('[data-modal="set-password"] input')
  else if (props.showChangeSelfPassword) focusFirst('[data-modal="change-password"] input')
})

onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  document.body.style.overflow = ''
})
</script>

<template>
  <div
    v-if="showCreate"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
    role="dialog"
    aria-modal="true"
    data-modal="create-user"
    @click.self="emit('update:showCreate', false)"
  >
    <div class="w-full max-w-sm rounded-xl bg-white p-5 shadow-xl">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">新建用户</h3>
      <div class="space-y-2">
        <input :value="newUser.name" @input="emit('update:newUser', { ...newUser, name: ($event.target as HTMLInputElement).value })" placeholder="用户名" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" />
        <input :value="newUser.display_name" @input="emit('update:newUser', { ...newUser, display_name: ($event.target as HTMLInputElement).value })" placeholder="显示名 (可选)" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" />
        <select :value="newUser.role" @change="emit('update:newUser', { ...newUser, role: ($event.target as HTMLSelectElement).value })" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none">
          <option value="user">普通用户</option>
          <option value="admin">管理员</option>
        </select>
        <input :value="newUser.password" @input="emit('update:newUser', { ...newUser, password: ($event.target as HTMLInputElement).value })" type="password" placeholder="密码 (可选)" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" />
      </div>
      <div class="mt-4 flex justify-end gap-2">
        <button class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm" @click="emit('update:showCreate', false)">取消</button>
        <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="emit('create-user')">创建</button>
      </div>
    </div>
  </div>

  <div
    v-if="showSetPassword"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
    role="dialog"
    aria-modal="true"
    data-modal="set-password"
    @click.self="emit('update:showSetPassword', false)"
  >
    <div class="w-full max-w-sm rounded-xl bg-white p-5 shadow-xl">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">设置密码 · {{ setPasswordUser }}</h3>
      <input :value="setPasswordValue" @input="emit('update:setPasswordValue', ($event.target as HTMLInputElement).value)" type="password" placeholder="输入新密码" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" @keyup.enter="emit('set-password')" />
      <div class="mt-4 flex justify-end gap-2">
        <button class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm" @click="emit('update:showSetPassword', false)">取消</button>
        <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="emit('set-password')">设置</button>
      </div>
    </div>
  </div>

  <div
    v-if="showChangeSelfPassword"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
    role="dialog"
    aria-modal="true"
    data-modal="change-password"
    @click.self="emit('update:showChangeSelfPassword', false)"
  >
    <div class="w-full max-w-sm rounded-xl bg-white p-5 shadow-xl">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">修改我的密码</h3>
      <div class="space-y-2">
        <input :value="selfOldPassword" @input="emit('update:selfOldPassword', ($event.target as HTMLInputElement).value)" type="password" placeholder="当前密码" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" />
        <input :value="selfNewPassword" @input="emit('update:selfNewPassword', ($event.target as HTMLInputElement).value)" type="password" placeholder="新密码" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" @keyup.enter="emit('change-password')" />
      </div>
      <div class="mt-4 flex justify-end gap-2">
        <button class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm" @click="emit('update:showChangeSelfPassword', false)">取消</button>
        <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="emit('change-password')">确认修改</button>
      </div>
    </div>
  </div>
</template>
