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
import { sessionClockTickMs } from '@/utils/sessionClock'

function createSessionClock(opts?: { tickMs?: number; fineTickMs?: number }) {
  const nowMs = ref(Date.now())
  const defaultTickMs = opts?.tickMs ?? 15_000
  const fineTickMs = opts?.fineTickMs ?? 1_000
  let timer: ReturnType<typeof setInterval> | null = null
  let currentIntervalMs = 0

  const sessionView = computed(() => {
    const exp = parseExpiresAt(getTokenExpiresAt())
    return sessionExpiryView(exp, nowMs.value)
  })

  function clearTimer() {
    if (timer) clearInterval(timer)
    timer = null
    currentIntervalMs = 0
  }

  function desiredTickMs() {
    return sessionClockTickMs(
      sessionView.value.urgency,
      sessionView.value.remainingMs,
      defaultTickMs,
      fineTickMs,
    )
  }

  function scheduleTimer() {
    const ms = desiredTickMs()
    if (timer && currentIntervalMs === ms) return
    clearTimer()
    currentIntervalMs = ms
    timer = setInterval(() => {
      nowMs.value = Date.now()
      scheduleTimer()
    }, ms)
  }

  function start() {
    nowMs.value = Date.now()
    scheduleTimer()
  }

  return { nowMs, sessionView, start, stop: clearTimer }
}

/**
 * 页面级变更写门禁：浏览器离线 或 会话 critical/expired。
 * 登录/会话密码续期应继续仅用 offline，勿使用 writeBlocked。
 * 临界态时钟默认 1s 刷新，其它态 15s。
 */
export function useMutationGuard(opts?: { tickMs?: number; fineTickMs?: number }) {
  const { offline } = useNetworkStatus()
  const clock = createSessionClock(opts)

  const sessionUrgency = computed<SessionUrgency>(() => clock.sessionView.value.urgency)
  const sessionRemainingLabel = computed(() => clock.sessionView.value.label || '')
  const writeBlocked = computed(() => shouldBlockMutation(offline.value, sessionUrgency.value))
  const blockReason = computed<MutationBlockReason>(() =>
    mutationBlockReason(offline.value, sessionUrgency.value),
  )
  const sessionWriteBlocked = computed(() => blockReason.value === 'session')

  onMounted(() => clock.start())
  onBeforeUnmount(() => clock.stop())

  function blockedMessageKey(offlineKey: MessageKey): MessageKey {
    return mutationBlockedMessageKey(blockReason.value, offlineKey)
  }

  return {
    offline,
    sessionView: clock.sessionView,
    sessionUrgency,
    sessionRemainingLabel,
    sessionWriteBlocked,
    writeBlocked,
    blockReason,
    blockedMessageKey,
    shouldBlock: () => writeBlocked.value,
  }
}
