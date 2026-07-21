import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { getTokenExpiresAt } from '@/api/client'
import { useNetworkStatus } from '@/composables/useNetworkStatus'
import {
  mutationBlockReason,
  shouldBlockMutation,
  type MutationBlockReason,
} from '@/utils/mutationGuard'
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

  const sessionUrgency = computed<SessionUrgency>(() => {
    const exp = parseExpiresAt(getTokenExpiresAt())
    return sessionExpiryView(exp, nowMs.value).urgency
  })

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

  return {
    offline,
    sessionUrgency,
    sessionWriteBlocked,
    writeBlocked,
    blockReason,
    shouldBlock: () => writeBlocked.value,
  }
}
