/** 从服务端拉取并应用密码策略（失败静默，保留本地默认） */

import { fetchPasswordPolicy } from '@/api/client'
import { applyServerPasswordPolicy, type ServerPasswordPolicy } from '@/utils/passwordPolicy'

let inflight: Promise<boolean> | null = null

export async function bootstrapPasswordPolicy(signal?: AbortSignal): Promise<boolean> {
  if (inflight) return inflight
  inflight = (async () => {
    try {
      const data = await fetchPasswordPolicy(signal ? { signal } : {})
      applyServerPasswordPolicy(data as ServerPasswordPolicy)
      return true
    } catch {
      return false
    } finally {
      inflight = null
    }
  })()
  return inflight
}
