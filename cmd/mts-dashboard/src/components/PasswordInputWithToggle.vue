<script setup lang="ts">
import { ref } from 'vue'
import { Eye, EyeOff } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'

const model = defineModel<string>({ default: '' })
const emit = defineEmits<{ enter: [] }>()

withDefaults(
  defineProps<{
    id?: string
    autocomplete?: string
    disabled?: boolean
    invalid?: boolean
    describedBy?: string
    testId?: string
    toggleTestId?: string
    inputClass?: string
    placeholder?: string
    name?: string
  }>(),
  {
    autocomplete: 'current-password',
    disabled: false,
    invalid: false,
    inputClass: 'mts-input mts-focus-ring pr-10',
  },
)

const { t } = useI18n()
const show = ref(false)
</script>

<template>
  <div class="relative">
    <input
      :id="id"
      v-model="model"
      :type="show ? 'text' : 'password'"
      :autocomplete="autocomplete"
      :disabled="disabled"
      :class="inputClass"
      :data-testid="testId"
      :aria-invalid="invalid ? 'true' : undefined"
      :aria-describedby="describedBy"
      :placeholder="placeholder"
      :name="name"
      @keyup.enter="emit('enter')"
    />
    <button
      type="button"
      class="mts-focus-ring absolute inset-y-0 right-1 my-auto inline-flex h-8 w-8 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
      :data-testid="toggleTestId"
      :aria-label="show ? t('loginHidePassword') : t('loginShowPassword')"
      :title="show ? t('loginHidePassword') : t('loginShowPassword')"
      :aria-pressed="show ? 'true' : 'false'"
      :disabled="disabled"
      @click="show = !show"
    >
      <EyeOff v-if="show" class="h-4 w-4" aria-hidden="true" />
      <Eye v-else class="h-4 w-4" aria-hidden="true" />
    </button>
  </div>
</template>
