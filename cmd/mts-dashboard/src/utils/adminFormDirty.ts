/** 管理页创建表单脏状态（纯函数） */

export type UserCreateDraft = {
  name?: string
  display_name?: string
  password?: string
  role?: string
}

export type DownsampleCreateDraft = {
  name?: string
  source_database?: string
  source_measurement?: string
  target_database?: string
  target_measurement?: string
  interval_human?: string
  functions_json?: string
  group_by_tags?: string
  enabled?: boolean
}

function nonEmpty(s: string | null | undefined): boolean {
  return typeof s === 'string' && s.trim().length > 0
}

/** 用户创建表单是否相对空草稿有改动 */
export function isUserCreateDraftDirty(
  draft: UserCreateDraft | null | undefined,
  defaults?: { role?: string },
): boolean {
  if (!draft) return false
  const defaultRole = defaults?.role ?? 'user'
  if (nonEmpty(draft.name)) return true
  if (nonEmpty(draft.display_name)) return true
  if (nonEmpty(draft.password)) return true
  if (nonEmpty(draft.role) && draft.role !== defaultRole) return true
  return false
}

/** 设密 / 自改密表单是否有未提交输入 */
export function isPasswordDraftDirty(password: string | null | undefined, extra?: string | null): boolean {
  return nonEmpty(password) || nonEmpty(extra)
}

/** 降采样创建表单是否相对空草稿有改动 */
export function isDownsampleCreateDraftDirty(
  draft: DownsampleCreateDraft | null | undefined,
  defaults?: { interval_human?: string; enabled?: boolean },
): boolean {
  if (!draft) return false
  const intervalDefault = defaults?.interval_human ?? '1m'
  const enabledDefault = defaults?.enabled ?? true
  if (nonEmpty(draft.name)) return true
  if (nonEmpty(draft.source_database)) return true
  if (nonEmpty(draft.source_measurement)) return true
  if (nonEmpty(draft.target_database)) return true
  if (nonEmpty(draft.target_measurement)) return true
  const intervalHuman = draft.interval_human ?? ''
  if (nonEmpty(intervalHuman) && intervalHuman.trim() !== intervalDefault) return true
  if (nonEmpty(draft.group_by_tags)) return true
  const functionsJSON = draft.functions_json ?? ''
  if (nonEmpty(functionsJSON) && functionsJSON !== '[]') {
    try {
      const arr = JSON.parse(functionsJSON) as Array<{ field?: string; as?: string; function?: string }>
      if (Array.isArray(arr)) {
        for (const f of arr) {
          if (nonEmpty(f?.field) && f.field !== 'value') return true
          if (nonEmpty(f?.as) && f.as !== 'mean_value') return true
          if (nonEmpty(f?.function) && f.function !== 'mean') return true
        }
        if (arr.length !== 1) return true
      }
    } catch {
      return true
    }
  }
  if (typeof draft.enabled === 'boolean' && draft.enabled !== enabledDefault) return true
  return false
}

/** 打开创建面板且草稿脏时，应拦截离开 */
export function shouldBlockLeaveAdminCreate(open: boolean, draftDirty: boolean): boolean {
  return open === true && draftDirty === true
}
