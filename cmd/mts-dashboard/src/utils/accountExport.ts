/** 账户会话与资料导出（纯函数，不含密钥） */

export function buildAccountExport(
  input: {
    username?: string
    role?: string
    session?: {
      expires_at?: string | null
      remaining?: string
      urgency?: string
    } | null
  },
  at = new Date(),
): {
  kind: 'mts.account.snapshot'
  version: 1
  generated_at: string
  username: string
  role: string
  session: {
    expires_at: string
    remaining: string
    urgency: string
  }
} {
  const session = input.session || {}
  return {
    kind: 'mts.account.snapshot',
    version: 1,
    generated_at: at.toISOString(),
    username: input.username || '',
    role: input.role || '',
    session: {
      expires_at: session.expires_at || '',
      remaining: session.remaining || '',
      urgency: session.urgency || '',
    },
  }
}

export function formatAccountExportPretty(
  input: Parameters<typeof buildAccountExport>[0],
  at = new Date(),
): string {
  return JSON.stringify(buildAccountExport(input, at), null, 2)
}
