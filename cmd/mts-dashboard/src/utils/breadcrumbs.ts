/** 路由面包屑（纯函数） */

export interface BreadcrumbItem {
  path: string
  labelKey: string
}

/** 路径段 -> i18n key（相对 message key） */
const SEGMENT_KEYS: Record<string, string> = {
  '': 'overview',
  databases: 'databases',
  users: 'users',
  config: 'config',
  operations: 'operations',
  downsample: 'downsample',
  query: 'query',
  audit: 'audit',
  'api-spec': 'apiSpec',
  storage: 'storage',
  about: 'about',
  account: 'account',
  write: 'write',
  access: 'accessMatrix',
  grants: 'accessGrants',
  ops: 'operations',
  readiness: 'readiness',
  observability: 'metrics',
  metrics: 'metrics',
}

export function buildBreadcrumbs(pathname: string): BreadcrumbItem[] {
  const raw = String(pathname || '/').split('?')[0].split('#')[0]
  const parts = raw.split('/').filter(Boolean)
  if (!parts.length) {
    return [{ path: '/', labelKey: 'overview' }]
  }
  const items: BreadcrumbItem[] = [{ path: '/', labelKey: 'overview' }]
  let acc = ''
  for (const seg of parts) {
    acc += `/${seg}`
    const key = SEGMENT_KEYS[seg]
    if (!key) {
      items.push({ path: acc, labelKey: 'notFound' })
      continue
    }
    // /ops 本身不作为独立可点页时仍给出中间节点
    if (seg === 'ops' || seg === 'observability') {
      items.push({ path: acc, labelKey: key })
      continue
    }
    items.push({ path: acc, labelKey: key })
  }
  // 去重连续相同 path
  return items.filter((it, i, arr) => i === 0 || it.path !== arr[i - 1].path)
}
