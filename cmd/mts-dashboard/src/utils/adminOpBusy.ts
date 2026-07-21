/** admin_op_busy 轮询辅助（纯函数） */

export function shouldPollAdminOpBusy(
  isAuthenticated: boolean | null | undefined,
  isAdmin: boolean | null | undefined,
): boolean {
  return Boolean(isAuthenticated) && Boolean(isAdmin)
}

export function parseAdminOpBusyPayload(payload: { admin_op_busy?: unknown } | null | undefined): boolean {
  return Boolean(payload?.admin_op_busy)
}
