/** 账户着陆页选项展示（纯函数） */

export type LandingOptionView = {
  path: string
  label: string
  adminOnly: boolean
}

export function buildLandingOptionViews(
  paths: readonly string[],
  labelOf: (path: string) => string,
  isAdminOnly: (path: string) => boolean,
): LandingOptionView[] {
  return paths.map((path) => ({
    path,
    label: labelOf(path),
    adminOnly: isAdminOnly(path),
  }))
}

/** 按 label 模糊筛选；空关键字返回全部 */
export function filterLandingOptions(
  items: LandingOptionView[],
  query: string,
): LandingOptionView[] {
  const q = query.trim().toLowerCase()
  if (!q) return [...items]
  return items.filter(
    (it) =>
      it.path.toLowerCase().includes(q) ||
      it.label.toLowerCase().includes(q),
  )
}

export function groupLandingOptions(items: LandingOptionView[]): {
  common: LandingOptionView[]
  admin: LandingOptionView[]
} {
  const common: LandingOptionView[] = []
  const admin: LandingOptionView[] = []
  for (const it of items) {
    if (it.adminOnly) admin.push(it)
    else common.push(it)
  }
  return { common, admin }
}
