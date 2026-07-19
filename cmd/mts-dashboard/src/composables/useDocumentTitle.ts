import { watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from '@/composables/useI18n'
import { documentTitleForRoute } from '@/utils/pageTitle'

/** 在根组件挂载：随路由与语言更新 document.title */
export function useDocumentTitle() {
  const route = useRoute()
  const { locale } = useI18n()

  function apply() {
    if (typeof document === 'undefined') return
    document.title = documentTitleForRoute(route.name, locale.value)
  }

  watch(() => [route.name, locale.value] as const, apply, { immediate: true })
}
