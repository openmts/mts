import { computed, ref } from 'vue'
import { makeActionResult, type ActionResult } from './actionResult.ts'
import { formatCaughtError } from './apiError.ts'

/** 写操作失败可重试状态机（无页面耦合） */
export function createActionRetry<K extends string>(opts?: {
  notifyError?: (msg: string) => void
}) {
  const lastFailedAction = ref<string | null>(null)
  const actionResult = ref<ActionResult | null>(null)
  const actionContext = ref<Record<string, string>>({})

  const canRetryAction = computed(
    () => !!(actionResult.value && actionResult.value.kind === 'error' && lastFailedAction.value),
  )

  function clearActionResult() {
    actionResult.value = null
    lastFailedAction.value = null
  }

  function setActionResult(result: ActionResult) {
    if (result.kind === 'ok' || result.kind === 'info' || result.kind === 'warn') {
      lastFailedAction.value = null
    }
    actionResult.value = result
  }

  function setActionOk(message: string) {
    lastFailedAction.value = null
    actionResult.value = makeActionResult('ok', message)
  }

  function setActionError(message: string) {
    lastFailedAction.value = null
    actionResult.value = makeActionResult('error', message)
  }

  function reportActionError(key: K, e: unknown, ctx?: Record<string, string>) {
    lastFailedAction.value = key
    actionContext.value = ctx ? { ...ctx } : {}
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    opts?.notifyError?.(msg)
  }

  return {
    lastFailedAction: lastFailedAction as { value: K | null },
    actionResult,
    actionContext,
    canRetryAction,
    clearActionResult,
    setActionResult,
    setActionOk,
    setActionError,
    reportActionError,
  }
}
