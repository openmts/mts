import { computed, ref, watch } from 'vue'
import { messages, type Locale, type MessageKey } from '@/i18n/messages'

const LOCALE_KEY = 'mts_locale'

function readLocale(): Locale {
  try {
    const v = localStorage.getItem(LOCALE_KEY)
    if (v === 'zh' || v === 'en') return v
  } catch { /* ignore */ }
  return 'zh'
}

const locale = ref<Locale>(readLocale())

watch(locale, (v) => {
  try { localStorage.setItem(LOCALE_KEY, v) } catch { /* ignore */ }
  document.documentElement.lang = v === 'zh' ? 'zh-CN' : 'en'
})

document.documentElement.lang = locale.value === 'zh' ? 'zh-CN' : 'en'

export function useI18n() {
  const t = computed(() => {
    const table = messages[locale.value]
    return (key: MessageKey): string => table[key] ?? key
  })
  function setLocale(next: Locale) {
    locale.value = next
  }
  function toggleLocale() {
    locale.value = locale.value === 'zh' ? 'en' : 'zh'
  }
  return { locale, t, setLocale, toggleLocale }
}
