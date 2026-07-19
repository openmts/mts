/** 登录页会话原因文案 */

export function loginReasonMessage(reason: unknown, locale: 'zh' | 'en' = 'zh'): string {
  const r = typeof reason === 'string' ? reason : ''
  if (locale === 'en') {
    switch (r) {
      case 'session':
        return 'Session expired or invalid. Please sign in again.'
      case 'storage':
        return 'Signed out in another tab. Please sign in again.'
      case 'auth':
        return 'Authentication required.'
      default:
        return ''
    }
  }
  switch (r) {
    case 'session':
      return '登录已过期或会话失效，请重新登录。'
    case 'storage':
      return '会话已在其他标签页退出，请重新登录。'
    case 'auth':
      return '请先登录后继续访问。'
    default:
      return ''
  }
}
