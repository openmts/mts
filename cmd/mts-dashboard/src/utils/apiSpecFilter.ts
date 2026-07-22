/** API Spec 筛选（纯函数：ns + 搜索 + 跨 ns fallback） */

export interface APISpecFilterEndpoint {
  method: string
  path: string
  auth?: string
  description?: string
  response?: string
}

export interface APISpecFilterNamespace {
  name: string
  base_path?: string
  endpoints: APISpecFilterEndpoint[]
}

export function endpointMatchesQuery(ep: APISpecFilterEndpoint, q: string): boolean {
  const text = q.trim().toLowerCase()
  if (!text) return true
  return (
    ep.method.toLowerCase().includes(text) ||
    ep.path.toLowerCase().includes(text) ||
    (ep.description || '').toLowerCase().includes(text) ||
    (ep.response || '').toLowerCase().includes(text) ||
    (ep.auth || '').toLowerCase().includes(text)
  )
}

/**
 * 按 namespace 与搜索词筛选。
 * 有搜索词且当前 ns 无命中时，自动跨命名空间展示，避免默认首个 ns 漏结果。
 */
export function filterApiSpecNamespaces(
  namespaces: APISpecFilterNamespace[] | null | undefined,
  opts: { q?: string; ns?: string } = {},
): APISpecFilterNamespace[] {
  const list = Array.isArray(namespaces) ? namespaces : []
  const text = (opts.q || '').trim().toLowerCase()
  const nsFilter = (opts.ns || '').trim()

  const mapScoped = (onlyNs: string | null) =>
    list
      .filter((ns) => !onlyNs || ns.name === onlyNs)
      .map((ns) => ({
        ...ns,
        endpoints: (ns.endpoints || []).filter((ep) => endpointMatchesQuery(ep, text)),
      }))
      .filter((ns) => ns.endpoints.length || !text)

  const scoped = mapScoped(nsFilter || null)
  if (text && nsFilter && !scoped.some((ns) => ns.endpoints.length)) {
    return mapScoped(null).filter((ns) => ns.endpoints.length)
  }
  return scoped
}
