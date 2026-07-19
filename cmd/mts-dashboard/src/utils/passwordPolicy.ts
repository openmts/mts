/** 前端改密策略（与 bootstrap 门禁一致，纯函数可单测） */

export interface PasswordPolicyResult {
  ok: boolean
  error?: string
}

export const MIN_PASSWORD_LENGTH = 8
export const FORBIDDEN_DEFAULT_PASSWORD = 'admin'

export function validateNewPassword(
  oldPassword: string,
  newPassword: string,
  confirmPassword: string,
  opts?: { locale?: 'zh' | 'en' },
): PasswordPolicyResult {
  const zh = (opts?.locale ?? 'zh') === 'zh'
  if (!oldPassword || !newPassword) {
    return { ok: false, error: zh ? '请填写旧密码与新密码' : 'Old and new password are required' }
  }
  if (newPassword.length < MIN_PASSWORD_LENGTH) {
    return {
      ok: false,
      error: zh ? `新密码至少 ${MIN_PASSWORD_LENGTH} 位` : `New password must be at least ${MIN_PASSWORD_LENGTH} characters`,
    }
  }
  if (newPassword === FORBIDDEN_DEFAULT_PASSWORD) {
    return { ok: false, error: zh ? '不能继续使用默认密码 admin' : 'Cannot keep default password admin' }
  }
  if (newPassword !== confirmPassword) {
    return { ok: false, error: zh ? '两次输入的新密码不一致' : 'Password confirmation does not match' }
  }
  if (newPassword === oldPassword) {
    return { ok: false, error: zh ? '新密码不能与旧密码相同' : 'New password must differ from old password' }
  }
  return { ok: true }
}
