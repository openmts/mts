/** 剪贴板写入（纯函数接口 + DOM 实现，便于单测 mock） */

export async function copyText(
  text: string,
  opts?: {
    writeText?: (value: string) => Promise<void>
  },
): Promise<{ ok: boolean; method: 'clipboard' | 'execCommand' | 'none'; error?: string }> {
  const value = String(text ?? '')
  if (!value) return { ok: false, method: 'none', error: 'empty' }

  const write = opts?.writeText
  if (write) {
    try {
      await write(value)
      return { ok: true, method: 'clipboard' }
    } catch (e) {
      return { ok: false, method: 'clipboard', error: e instanceof Error ? e.message : String(e) }
    }
  }

  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value)
      return { ok: true, method: 'clipboard' }
    } catch {
      // fallback below
    }
  }

  if (typeof document === 'undefined') {
    return { ok: false, method: 'none', error: 'no-document' }
  }

  try {
    const ta = document.createElement('textarea')
    ta.value = value
    ta.setAttribute('readonly', 'true')
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    ta.remove()
    return ok ? { ok: true, method: 'execCommand' } : { ok: false, method: 'execCommand', error: 'execCommand-failed' }
  } catch (e) {
    return { ok: false, method: 'execCommand', error: e instanceof Error ? e.message : String(e) }
  }
}
