function normalizeBasePath(baseURL: string): string {
  const trimmed = baseURL.trim()
  if (!trimmed || trimmed === '/') return '/'
  return `/${trimmed.replace(/^\/+|\/+$/g, '')}/`
}

/** 将应用内路径转换为感知部署前缀的绝对 URL。 */
export function buildAbsoluteAppURL(origin: string, baseURL: string, appPath: string): string {
  const normalizedOrigin = origin.replace(/\/+$/g, '')
  const basePath = normalizeBasePath(baseURL)
  const normalizedAppPath = `/${appPath.replace(/^\/+/, '')}`
  const appPathname = normalizedAppPath.split(/[?#]/, 1)[0]
  const baseRoot = basePath === '/' ? '/' : basePath.slice(0, -1)
  const alreadyPrefixed = appPathname === baseRoot || normalizedAppPath.startsWith(basePath)
  const prefixedPath =
    basePath !== '/' && alreadyPrefixed
      ? normalizedAppPath
      : `${basePath === '/' ? '' : basePath.slice(0, -1)}${normalizedAppPath}`
  return `${normalizedOrigin}${prefixedPath}`
}

export function buildShareURL(appPath: string): string {
  return buildAbsoluteAppURL(window.location.origin, import.meta.env.BASE_URL, appPath)
}
