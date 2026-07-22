/** 前端密码策略（与 server password_policy 对齐，纯函数可单测） */

export interface PasswordPolicyResult {
  ok: boolean
  error?: string
}

export const MIN_PASSWORD_LENGTH = 8
export const FORBIDDEN_DEFAULT_PASSWORD = 'admin'

export type PasswordLocale = 'zh' | 'en'

/** 服务端公开策略快照（GET /api/v1/auth/password-policy） */
export interface ServerPasswordPolicy {
  ok?: boolean
  min_length?: number
  forbidden_defaults?: string[]
  require_change_bootstrap?: boolean
  version?: number
}

let runtimeMinLength = MIN_PASSWORD_LENGTH
let runtimeForbidden: string[] = [FORBIDDEN_DEFAULT_PASSWORD]
let runtimeVersion = 0
let runtimeRequireBootstrap = true

export function getMinPasswordLength(): number {
  return runtimeMinLength
}

export function getForbiddenDefaultPasswords(): string[] {
  return runtimeForbidden.slice()
}

export function getPasswordPolicyVersion(): number {
  return runtimeVersion
}

export function getRequireChangeBootstrap(): boolean {
  return runtimeRequireBootstrap
}

/** 应用服务端策略；非法字段忽略并保留默认。测试可 resetPasswordPolicyRuntime。 */
export function applyServerPasswordPolicy(input: ServerPasswordPolicy | null | undefined): void {
  if (!input || typeof input !== 'object') return
  const min = input.min_length
  if (typeof min === 'number' && Number.isFinite(min)) {
    const n = Math.floor(min)
    if (n >= 4 && n <= 128) runtimeMinLength = n
  }
  if (Array.isArray(input.forbidden_defaults) && input.forbidden_defaults.length) {
    const next = input.forbidden_defaults
      .map((x) => String(x ?? '').trim())
      .filter((x) => x.length > 0)
    if (next.length) runtimeForbidden = next
  }
  if (typeof input.version === 'number' && Number.isFinite(input.version) && input.version > 0) {
    runtimeVersion = Math.floor(input.version)
  }
  if (typeof input.require_change_bootstrap === 'boolean') {
    runtimeRequireBootstrap = input.require_change_bootstrap
  }
}

export function resetPasswordPolicyRuntime(): void {
  runtimeMinLength = MIN_PASSWORD_LENGTH
  runtimeForbidden = [FORBIDDEN_DEFAULT_PASSWORD]
  runtimeVersion = 0
  runtimeRequireBootstrap = true
}

function isZh(locale?: PasswordLocale): boolean {
  return (locale ?? 'zh') === 'zh'
}

function isForbiddenPassword(p: string): boolean {
  return runtimeForbidden.includes(p)
}

/** 管理员设置 / 创建用户时的密码强度（allowEmpty 用于创建时可选密码） */
export function validateAssignedPassword(
  password: string,
  opts?: { locale?: PasswordLocale; allowEmpty?: boolean },
): PasswordPolicyResult {
  const zh = isZh(opts?.locale)
  const p = password || ''
  const minLen = getMinPasswordLength()
  if (!p) {
    if (opts?.allowEmpty) return { ok: true }
    return { ok: false, error: zh ? '请填写密码' : 'Password is required' }
  }
  if (isForbiddenPassword(p)) {
    return { ok: false, error: zh ? '不能使用默认密码 admin' : 'Cannot use default password admin' }
  }
  if (p.length < minLen) {
    return {
      ok: false,
      error: zh
        ? `密码至少 ${minLen} 位`
        : `Password must be at least ${minLen} characters`,
    }
  }
  return { ok: true }
}

/**
 * 自助改密策略。
 * requireConfirm 默认 true；Users 自改密弹窗无确认框时传 false。
 */
export function validateNewPassword(
  oldPassword: string,
  newPassword: string,
  confirmPassword: string,
  opts?: { locale?: PasswordLocale; requireConfirm?: boolean },
): PasswordPolicyResult {
  const zh = isZh(opts?.locale)
  const requireConfirm = opts?.requireConfirm !== false
  if (!oldPassword || !newPassword) {
    return { ok: false, error: zh ? '请填写旧密码与新密码' : 'Old and new password are required' }
  }
  const strength = validateAssignedPassword(newPassword, { locale: opts?.locale })
  if (!strength.ok) return strength
  if (requireConfirm && newPassword !== confirmPassword) {
    return { ok: false, error: zh ? '两次输入的新密码不一致' : 'Password confirmation does not match' }
  }
  if (newPassword === oldPassword) {
    return { ok: false, error: zh ? '新密码不能与旧密码相同' : 'New password must differ from old password' }
  }
  return { ok: true }
}
