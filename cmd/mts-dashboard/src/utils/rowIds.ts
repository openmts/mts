/** 列表行稳定主键（纯函数） */

export function auditRowId(
  evt: {
    time?: string
    user_name?: string
    action?: string
    database?: string
    detail?: string
  },
  index: number,
): string {
  const parts = [
    String(evt.time ?? ''),
    String(evt.user_name ?? ''),
    String(evt.action ?? ''),
    String(evt.database ?? ''),
    String(evt.detail ?? ''),
    String(index),
  ]
  return parts.join('\u001f')
}

export function grantRowId(row: {
  user?: string
  database?: string
  permission?: string
}): string {
  return [String(row.user ?? ''), String(row.database ?? ''), String(row.permission ?? '')].join(
    '\u001f',
  )
}
