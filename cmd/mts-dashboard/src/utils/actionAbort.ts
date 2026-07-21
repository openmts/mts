/** 可取消的页面动作：AbortController 生命周期 */

export function createActionAbort() {
  let controller: AbortController | null = null

  function cancel() {
    if (!controller) return
    try {
      controller.abort()
    } catch {
      /* ignore */
    }
    controller = null
  }

  function begin(): AbortSignal {
    cancel()
    controller = new AbortController()
    return controller.signal
  }

  function end() {
    controller = null
  }

  function active(): boolean {
    return controller != null
  }

  return { cancel, begin, end, active }
}
