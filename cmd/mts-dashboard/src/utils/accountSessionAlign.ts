/** Account 页会话/密码策略 path 对齐摘要（纯函数） */

export const AUTH_SESSION_PATH = '/api/v1/auth/session'
export const AUTH_LOGIN_PATH = '/api/v1/auth/login'
export const AUTH_LOGOUT_PATH = '/api/v1/auth/logout'
export const AUTH_PASSWORD_POLICY_PATH = '/api/v1/auth/password-policy'
export const AUTH_CHANGE_PASSWORD_PATH = '/api/v1/auth/change-password'

export interface AccountSessionAlign {
  session_path: string
  login_path: string
  logout_path: string
  password_policy_path: string
  change_password_path: string
  sample_source: 'login' | 'session' | ''
  has_server_remaining: boolean
  has_local_expiry: boolean
  urgency: string
  path_ok: boolean
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

export function alignAccountSession(input: {
  sessionPath?: string | null
  loginPath?: string | null
  logoutPath?: string | null
  passwordPolicyPath?: string | null
  changePasswordPath?: string | null
  sampleSource?: string | null
  hasServerRemaining?: boolean
  hasLocalExpiry?: boolean
  urgency?: string | null
}): AccountSessionAlign {
  const session_path = str(input.sessionPath, AUTH_SESSION_PATH)
  const login_path = str(input.loginPath, AUTH_LOGIN_PATH)
  const logout_path = str(input.logoutPath, AUTH_LOGOUT_PATH)
  const password_policy_path = str(input.passwordPolicyPath, AUTH_PASSWORD_POLICY_PATH)
  const change_password_path = str(input.changePasswordPath, AUTH_CHANGE_PASSWORD_PATH)
  const raw = String(input.sampleSource || '').trim()
  const sample_source: AccountSessionAlign['sample_source'] =
    raw === 'login' || raw === 'session' ? raw : ''
  const has_server_remaining = Boolean(input.hasServerRemaining)
  const has_local_expiry = Boolean(input.hasLocalExpiry)
  const urgency = String(input.urgency || '').trim() || 'unknown'
  const path_ok =
    session_path.includes('/auth/session') &&
    password_policy_path.includes('password-policy')
  let tone: AccountSessionAlign['tone'] = 'unknown'
  if (!path_ok) tone = 'bad'
  else if (urgency === 'critical' || urgency === 'expired') tone = 'bad'
  else if (urgency === 'warn' || !has_server_remaining) tone = 'warn'
  else if (path_ok && has_local_expiry) tone = 'ok'
  else tone = 'ok'
  return {
    session_path,
    login_path,
    logout_path,
    password_policy_path,
    change_password_path,
    sample_source,
    has_server_remaining,
    has_local_expiry,
    urgency,
    path_ok,
    tone,
  }
}

function str(v: unknown, fallback: string): string {
  const s = String(v || '').trim()
  return s || fallback
}
