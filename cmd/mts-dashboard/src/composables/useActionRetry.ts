import { createActionRetry } from '@/utils/actionRetry'

/** 页面 composable 封装：写操作失败可重试 */
export function useActionRetry<K extends string>(opts?: {
  notifyError?: (msg: string) => void
}) {
  return createActionRetry<K>(opts)
}
