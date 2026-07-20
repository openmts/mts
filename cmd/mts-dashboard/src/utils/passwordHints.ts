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
