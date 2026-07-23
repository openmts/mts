/** Access Matrix 静态能力矩阵扫视摘要（纯函数） */

export interface AccessMatrixMetaAlign {
  total_count: number
  filtered_count: number
  selected_count: number
  with_route_count: number
  admin_full_count: number
  user_data_count: number
  tone: 'ok' | 'warn' | 'unknown'
}

export function alignAccessMatrixMeta(input: {
  totalCount?: number
  filteredCount?: number
  selectedCount?: number
  withRouteCount?: number
  adminFullCount?: number
  userDataCount?: number
}): AccessMatrixMetaAlign {
  const total_count = finiteNonNeg(input.totalCount)
  const filtered_count = finiteNonNeg(input.filteredCount)
  const selected_count = finiteNonNeg(input.selectedCount)
  const with_route_count = finiteNonNeg(input.withRouteCount)
  const admin_full_count = finiteNonNeg(input.adminFullCount)
  const user_data_count = finiteNonNeg(input.userDataCount)
  let tone: AccessMatrixMetaAlign['tone'] = 'unknown'
  if (total_count > 0 && filtered_count === 0) tone = 'warn'
  else if (total_count > 0) tone = 'ok'
  return {
    total_count,
    filtered_count,
    selected_count,
    with_route_count,
    admin_full_count,
    user_data_count,
    tone,
  }
}

function finiteNonNeg(v: unknown): number {
  if (!Number.isFinite(Number(v))) return 0
  return Math.max(0, Math.trunc(Number(v)))
}
