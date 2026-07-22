/** 前端密码策略（与 server password_policy 对齐，纯函数可单测） */

export interface PasswordPolicyResult {
  ok: boolean
  error?: string
}

export const MIN_PASSWORD_LENGTH = 8
export const FORBIDDEN_DEFAULT_PASSWORD = 'admin'

export type PasswordLocale = 'zh' | 'en'

function isZh(locale?: PasswordLocale): boolean {
  return (locale ?? 'zh') === 'zh'
}

/** 管理员设置 / 创建用户时的密码强度（allowEmpty 用于创建时可选密码） */
export function validateAssignedPassword(
  password: string,
  opts?: { locale?: PasswordLocale; allowEmpty?: boolean },
): PasswordPolicyResult {
  const zh = isZh(opts?.locale)
  const p = password || ''
  if (!p) {
    if (opts?.allowEmpty) return { ok: true }
    return { ok: false, error: zh ? '请填写密码' : 'Password is required' }
  }
  if (p === FORBIDDEN_DEFAULT_PASSWORD) {
    return { ok: false, error: zh ? '不能使用默认密码 admin' : 'Cannot use default password admin' }
  }
  if (p.length < MIN_PASSWORD_LENGTH) {
    return {
      ok: false,
      error: zh
        ? `密码至少 ${MIN_PASSWORD_LENGTH} 位`
        : `Password must be at least ${MIN_PASSWORD_LENGTH} characters`,
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
