/** DashboardLayout 注册打开通知历史；NotifyHost 等跨布局组件可请求打开 */

type Opener = () => void

let opener: Opener | null = null

export function registerOpenNotifyHistory(fn: Opener): () => void {
  opener = fn
  return () => {
    if (opener === fn) opener = null
  }
}

export function requestOpenNotifyHistory(): boolean {
  if (!opener) return false
  opener()
  return true
}

export function hasOpenNotifyHistory(): boolean {
  return opener != null
}
