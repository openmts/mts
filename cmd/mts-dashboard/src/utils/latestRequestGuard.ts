export type LatestRequestToken = {
  commit: (apply: () => void) => boolean
}

export type LatestRequestGuard = {
  begin: () => LatestRequestToken
}

export function beginLatestLoad(
  guard: LatestRequestGuard,
  hasTarget: boolean,
  setLoading: (loading: boolean) => void,
): LatestRequestToken | null {
  const token = guard.begin()
  setLoading(hasTarget)
  return hasTarget ? token : null
}

/** 仅允许同一加载流程中最后开始的请求提交状态。 */
export function createLatestRequestGuard(): LatestRequestGuard {
  let sequence = 0

  return {
    begin() {
      const current = ++sequence
      return {
        commit(apply) {
          if (current !== sequence) return false
          apply()
          return true
        },
      }
    },
  }
}
