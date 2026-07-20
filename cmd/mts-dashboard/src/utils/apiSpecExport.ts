/** API Spec 导出（纯函数） */

export interface APISpecEndpoint {
  method: string
  path: string
  auth?: string
  description?: string
}

export interface APISpecNamespace {
  name: string
  base_path?: string
  endpoints: APISpecEndpoint[]
}

export function buildApiSpecExport(
  input: {
    version?: string
    namespaces?: APISpecNamespace[] | null
  },
  at = new Date(),
): {
  kind: 'mts.api_spec'
  version: 1
  generated_at: string
  api_version: string
  namespace_count: number
  endpoint_count: number
  namespaces: APISpecNamespace[]
} {
  const namespaces = Array.isArray(input.namespaces) ? input.namespaces : []
  const endpoint_count = namespaces.reduce((n, ns) => n + (ns.endpoints?.length || 0), 0)
  return {
    kind: 'mts.api_spec',
    version: 1,
    generated_at: at.toISOString(),
    api_version: input.version || 'v1',
    namespace_count: namespaces.length,
    endpoint_count,
    namespaces,
  }
}

export function apiSpecToMarkdown(
  input: {
    version?: string
    namespaces?: APISpecNamespace[] | null
  },
  locale: 'zh' | 'en' = 'zh',
): string {
  const payload = buildApiSpecExport(input)
  const lines: string[] = []
  lines.push(locale === 'en' ? '# MTS API Spec' : '# MTS API 规范')
  lines.push('')
  lines.push(`${locale === 'en' ? 'API version' : 'API 版本'}: ${payload.api_version}`)
  lines.push(`${locale === 'en' ? 'Namespaces' : '命名空间'}: ${payload.namespace_count}`)
  lines.push(`${locale === 'en' ? 'Endpoints' : '端点'}: ${payload.endpoint_count}`)
  lines.push('')
  for (const ns of payload.namespaces) {
    lines.push(`## ${ns.name}`)
    if (ns.base_path) lines.push(`- base: \`${ns.base_path}\``)
    lines.push('')
    for (const ep of ns.endpoints || []) {
      lines.push(`- \`${ep.method} ${ep.path}\`${ep.auth ? ` (${ep.auth})` : ''}${ep.description ? ` — ${ep.description}` : ''}`)
    }
    lines.push('')
  }
  return lines.join('\n')
}
