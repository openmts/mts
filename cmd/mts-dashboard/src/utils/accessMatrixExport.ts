/** 权限矩阵导出（纯函数） */

import {
  ACCESS_LEVEL_LABEL,
  RBAC_CAPABILITY_MATRIX,
  type CapabilityRow,
  type LocaleCode,
  textForLocale,
} from './rbacMatrix.ts'
import { escapeCSVCell } from './csvEscape.ts'

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

/** 本地化矩阵 CSV（与 buildAccessMatrixExport 字段对齐） */
export function accessMatrixToCSV(
  rows: CapabilityRow[] | null | undefined,
  locale: LocaleCode = 'zh',
): string {
  const list = Array.isArray(rows) ? rows : []
  const header = ['id', 'area', 'capability', 'admin', 'user', 'route', 'notes']
  const lines = [header.join(',')]
  for (const r of list) {
    lines.push(
      [
        r.id,
        textForLocale(r.area, locale),
        textForLocale(r.capability, locale),
        textForLocale(ACCESS_LEVEL_LABEL[r.admin], locale),
        textForLocale(ACCESS_LEVEL_LABEL[r.user], locale),
        r.route || '',
        r.notes ? textForLocale(r.notes, locale) : '',
      ]
        .map(escapeCSVCell)
        .join(','),
    )
  }
  return lines.join('\n') + (lines.length ? '\n' : '')
}
