<script setup lang="ts">
interface NewUser { name: string; display_name: string; password: string; role: string }

const props = defineProps<{
  showCreate: boolean
  showSetPassword: boolean
  showChangeSelfPassword: boolean
  setPasswordUser: string
  currentUser: string
  newUser: NewUser
  setPasswordValue: string
  selfOldPassword: string
  selfNewPassword: string
}>()

const emit = defineEmits<{
  'update:showCreate': [boolean]
  'update:showSetPassword': [boolean]
  'update:showChangeSelfPassword': [boolean]
  'update:newUser': [NewUser]
  'update:setPasswordValue': [string]
  'update:selfOldPassword': [string]
  'update:selfNewPassword': [string]
  'create-user': []
  'set-password': []
  'change-password': []
}>()
</script>

<template>
  <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="emit('update:showCreate', false)">
    <div class="w-80 rounded-xl bg-white p-6 shadow-lg">
      <h3 class="mb-4 text-sm font-semibold text-slate-800">创建用户</h3>
      <div class="space-y-3">
        <input :value="newUser.name" @input="emit('update:newUser', { ...newUser, name: ($event.target as HTMLInputElement).value })" type="text" placeholder="用户名 (必填)" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" @keyup.enter="emit('create-user')" />
        <input :value="newUser.display_name" @input="emit('update:newUser', { ...newUser, display_name: ($event.target as HTMLInputElement).value })" type="text" placeholder="显示名 (可选)" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" />
        <select :value="newUser.role" @change="emit('update:newUser', { ...newUser, role: ($event.target as HTMLSelectElement).value })" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none">
          <option value="user">普通用户</option>
          <option value="admin">管理员</option>
        </select>
        <input :value="newUser.password" @input="emit('update:newUser', { ...newUser, password: ($event.target as HTMLInputElement).value })" type="password" placeholder="密码 (可选)" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" />
      </div>
      <div class="mt-4 flex justify-end gap-2">
        <button class="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100" @click="emit('update:showCreate', false)">取消</button>
        <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="emit('create-user')">创建</button>
      </div>
    </div>
  </div>
  <div v-if="showSetPassword" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="emit('update:showSetPassword', false)">
    <div class="w-80 rounded-xl bg-white p-6 shadow-lg">
      <h3 class="mb-4 text-sm font-semibold text-slate-800">设置密码 - {{ setPasswordUser }}</h3>
      <input :value="setPasswordValue" @input="emit('update:setPasswordValue', ($event.target as HTMLInputElement).value)" type="password" placeholder="输入新密码" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" @keyup.enter="emit('set-password')" />
      <div class="mt-4 flex justify-end gap-2">
        <button class="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100" @click="emit('update:showSetPassword', false)">取消</button>
        <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="emit('set-password')">设置</button>
      </div>
    </div>
  </div>
  <div v-if="showChangeSelfPassword" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="emit('update:showChangeSelfPassword', false)">
    <div class="w-80 rounded-xl bg-white p-6 shadow-lg">
      <h3 class="mb-4 text-sm font-semibold text-slate-800">修改密码 - {{ currentUser }}</h3>
      <div class="space-y-3">
        <input :value="selfOldPassword" @input="emit('update:selfOldPassword', ($event.target as HTMLInputElement).value)" type="password" placeholder="当前密码" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" />
        <input :value="selfNewPassword" @input="emit('update:selfNewPassword', ($event.target as HTMLInputElement).value)" type="password" placeholder="新密码" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none" @keyup.enter="emit('change-password')" />
      </div>
      <div class="mt-4 flex justify-end gap-2">
        <button class="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100" @click="emit('update:showChangeSelfPassword', false)">取消</button>
        <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="emit('change-password')">确认修改</button>
      </div>
    </div>
  </div>
</template>