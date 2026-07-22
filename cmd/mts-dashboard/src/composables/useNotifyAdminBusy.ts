/** 管理重操作互斥：统一 toast action + 乐观 busy */
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
import { useI18n } from '@/composables/useI18n'
import { useNotify } from '@/composables/useNotify'
import {
  adminOpBusyOpenAction,
  resolveAdminBusyNotify,
} from '@/utils/adminOpBusy'
import type { MessageKey } from '@/i18n/messages'

export function useNotifyAdminBusy(opts?: {
  /** 本地 busy / 无 message 时的默认文案 key */
  busyMessageKey?: MessageKey
}) {
  const { t } = useI18n()
  const { error: notifyError } = useNotify()
  const { adminOpBusy, setAdminOpBusy, refreshAdminOpBusy } = useAdminOpBusy()
  const busyKey = (opts?.busyMessageKey || 'opsAdminBusy') as MessageKey

  function notifyAdminBusy(message?: string) {
    const msg = String(message || '').trim() || t.value(busyKey)
    notifyError(msg, {
      action: adminOpBusyOpenAction(t.value('adminOpBusyOpenOps')),
    })
  }

  /** err 为 admin busy，或 treatLocalBusy 且当前本地 busy 时，带运维跳转 */
  function notifyMaybeAdminBusy(
    message: string,
    err?: unknown,
    flags?: { treatLocalBusy?: boolean },
  ) {
    const decision = resolveAdminBusyNotify(
      err,
      Boolean(flags?.treatLocalBusy && adminOpBusy.value),
    )
    if (decision.kind === 'admin_busy') {
      if (err != null) {
        setAdminOpBusy(true, decision.op || undefined)
        void refreshAdminOpBusy()
      }
      notifyAdminBusy(message)
      return
    }
    notifyError(message)
  }

  return {
    adminOpBusy,
    setAdminOpBusy,
    refreshAdminOpBusy,
    notifyAdminBusy,
    notifyMaybeAdminBusy,
  }
}
