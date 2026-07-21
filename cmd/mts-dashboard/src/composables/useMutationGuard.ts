import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { getTokenExpiresAt } from '@/api/client'
import { useNetworkStatus } from '@/composables/useNetworkStatus'
import {
  mutationBlockReason,
  mutationBlockedMessageKey,
  shouldBlockMutation,
  type MutationBlockReason,
} from '@/utils/mutationGuard'
import type { MessageKey } from '@/i18n/messages'
import { parseExpiresAt, sessionExpiryView, type SessionUrgency } from '@/utils/sessionExpiry'

/**
 * 页面级变更写门禁：浏览器离线 或 会话 critical/expired。
 * 登录/会话密码续期应继续仅用 offline，勿使用 writeBlocked。
 */
export function useMutationGuard(opts?: { tickMs?: number }) {
  const { offline } = useNetworkStatus()
  const nowMs = ref(Date.now())
  const tickMs = opts?.tickMs ?? 15_000
  let timer: ReturnType<typeof setInterval> | null = null

  const sessionView = computed(() => {
    const exp = parseExpiresAt(getTokenExpiresAt())
    return sessionExpiryView(exp, nowMs.value)
  })

  const sessionUrgency = computed<SessionUrgency>(() => sessionView.value.urgency)
  const sessionRemainingLabel = computed(() => sessionView.value.label || '')

  const writeBlocked = computed(() => shouldBlockMutation(offline.value, sessionUrgency.value))
  const blockReason = computed<MutationBlockReason>(() =>
    mutationBlockReason(offline.value, sessionUrgency.value),
  )
  const sessionWriteBlocked = computed(() => blockReason.value === 'session')

  function tick() {
    nowMs.value = Date.now()
  }

  onMounted(() => {
    tick()
    timer = setInterval(tick, tickMs)
  })
  onBeforeUnmount(() => {
    if (timer) clearInterval(timer)
    timer = null
  })

  /** 当前阻断原因对应的 i18n key（需配合 writeBlocked 使用） */
  function blockedMessageKey(offlineKey: MessageKey): MessageKey {
    return mutationBlockedMessageKey(blockReason.value, offlineKey)
  }

  return {
    offline,
    sessionView,
    sessionUrgency,
    sessionRemainingLabel,
    sessionWriteBlocked,
    writeBlocked,
    blockReason,
    blockedMessageKey,
    shouldBlock: () => writeBlocked.value,
  }
}
