/** 改密策略分项提示（纯函数，与 passwordPolicy 对齐） */

import { FORBIDDEN_DEFAULT_PASSWORD, MIN_PASSWORD_LENGTH } from './passwordPolicy.ts'

export interface PasswordHintItem {
  id: string
  ok: boolean
  labelKey: string
}

export function passwordRequirementHints(
  oldPassword: string,
  newPassword: string,
  confirmPassword: string,
): PasswordHintItem[] {
  const np = newPassword || ''
  const op = oldPassword || ''
  const cp = confirmPassword || ''
  return [
    {
      id: 'min_length',
      ok: np.length >= MIN_PASSWORD_LENGTH,
      labelKey: 'passwordHintMinLength',
    },
    {
      id: 'not_default',
      ok: np.length > 0 && np !== FORBIDDEN_DEFAULT_PASSWORD,
      labelKey: 'passwordHintNotDefault',
    },
    {
      id: 'diff_old',
      ok: np.length > 0 && op.length > 0 && np !== op,
      labelKey: 'passwordHintDiffOld',
    },
    {
      id: 'confirm_match',
      ok: np.length > 0 && cp.length > 0 && np === cp,
      labelKey: 'passwordHintConfirmMatch',
    },
  ]
}

export function passwordHintsAllOk(hints: PasswordHintItem[]): boolean {
  return hints.every((h) => h.ok)
}

/** 管理员设密 / 创建用户密码提示（无旧密码与确认） */
export function assignedPasswordHints(
  password: string,
  confirmPassword?: string,
): PasswordHintItem[] {
  const np = password || ''
  const cp = confirmPassword ?? ''
  const items: PasswordHintItem[] = [
    {
      id: 'min_length',
      ok: np.length >= MIN_PASSWORD_LENGTH,
      labelKey: 'passwordHintMinLength',
    },
    {
      id: 'not_default',
      ok: np.length > 0 && np !== FORBIDDEN_DEFAULT_PASSWORD,
      labelKey: 'passwordHintNotDefault',
    },
  ]
  if (confirmPassword !== undefined) {
    items.push({
      id: 'confirm_match',
      ok: np.length > 0 && cp.length > 0 && np === cp,
      labelKey: 'passwordHintConfirmMatch',
    })
  }
  return items
}

/** 策略分项完成度 0–100 */
export function passwordHintsProgress(hints: PasswordHintItem[]): {
  done: number
  total: number
  percent: number
} {
  const list = Array.isArray(hints) ? hints : []
  const total = list.length
  const done = list.filter((h) => h.ok).length
  const percent = total === 0 ? 0 : Math.round((done / total) * 100)
  return { done, total, percent }
}

