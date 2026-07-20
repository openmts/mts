/** 权限矩阵导出（纯函数） */

import {
  ACCESS_LEVEL_LABEL,
  RBAC_CAPABILITY_MATRIX,
  type CapabilityRow,
  type LocaleCode,
  textForLocale,
} from './rbacMatrix.ts'

export function buildAccessMatrixExport(
  rows: CapabilityRow[] | null | undefined = RBAC_CAPABILITY_MATRIX,
  locale: LocaleCode = 'zh',
  at = new Date(),
): {
  kind: 'mts.access.matrix'
  version: 1
  generated_at: string
  locale: LocaleCode
  count: number
  rows: Array<{
    id: string
    area: string
    capability: string
    admin: string
    user: string
    route?: string
    notes?: string
  }>
} {
  const list = Array.isArray(rows) ? rows : []
  return {
    kind: 'mts.access.matrix',
    version: 1,
    generated_at: at.toISOString(),
    locale,
    count: list.length,
    rows: list.map((r) => ({
      id: r.id,
      area: textForLocale(r.area, locale),
      capability: textForLocale(r.capability, locale),
      admin: textForLocale(ACCESS_LEVEL_LABEL[r.admin], locale),
      user: textForLocale(ACCESS_LEVEL_LABEL[r.user], locale),
      route: r.route,
      notes: r.notes ? textForLocale(r.notes, locale) : undefined,
    })),
  }
}
