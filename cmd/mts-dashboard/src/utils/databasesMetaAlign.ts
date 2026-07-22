/** Databases 页元数据路径/来源对齐摘要（纯函数） */

export type MetaListSource = 'admin' | 'data' | 'manual' | 'partial' | ''

export interface DatabasesMetaAlign {
  list_path: string
  source: MetaListSource
  database_count: number
  loaded_detail_count: number
  preferred_list_path: string
  source_ok: boolean
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

export function preferredDatabasesListPath(source: MetaListSource | string): string {
  if (source === 'admin') return '/api/v1/admin/databases'
  if (source === 'data') return '/api/v1/data/databases'
  return '/api/v1/data/databases'
}

export function alignDatabasesMeta(input: {
  listPath?: string | null
  source?: string | null
  databaseCount?: number
  loadedDetailCount?: number
}): DatabasesMetaAlign {
  const source = (String(input.source || '').trim() || '') as MetaListSource
  const list_path = String(input.listPath || '').trim() || preferredDatabasesListPath(source)
  const database_count = Number.isFinite(Number(input.databaseCount))
    ? Math.max(0, Math.trunc(Number(input.databaseCount)))
    : 0
  const loaded_detail_count = Number.isFinite(Number(input.loadedDetailCount))
    ? Math.max(0, Math.trunc(Number(input.loadedDetailCount)))
    : 0
  const preferred = preferredDatabasesListPath(source)
  let tone: DatabasesMetaAlign['tone'] = 'unknown'
  let source_ok = false
  if (source === 'manual') {
    tone = 'bad'
    source_ok = false
  } else if (source === 'partial') {
    tone = 'warn'
    source_ok = true
  } else if (source === 'admin' || source === 'data') {
    tone = 'ok'
    source_ok = true
  } else if (list_path) {
    tone = 'ok'
    source_ok = true
  }
  return {
    list_path,
    source: source || '',
    database_count,
    loaded_detail_count,
    preferred_list_path: preferred,
    source_ok,
    tone,
  }
}
