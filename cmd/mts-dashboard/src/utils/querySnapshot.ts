/** 查询结果区是否仍有可展示快照（失败保留判断） */

export function hasQueryResultSnapshot(input: {
  rows?: number
  columns?: number
  rawOutput?: string
  stats?: boolean
}): boolean {
  return (
    (input.rows ?? 0) > 0
    || (input.columns ?? 0) > 0
    || !!input.rawOutput
    || !!input.stats
  )
}
