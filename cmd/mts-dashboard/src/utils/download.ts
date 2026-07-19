/** 浏览器下载辅助（纯逻辑 + DOM 触发分离，便于单测） */

export function buildJSONBlob(payload: unknown): Blob {
  return new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
}

export function buildTextBlob(text: string, mime = 'text/plain'): Blob {
  return new Blob([text], { type: mime })
}

export function stampFilename(prefix: string, ext: string, at = new Date()): string {
  const stamp = at.toISOString().replace(/[:.]/g, '-').slice(0, 19)
  return `${prefix}-${stamp}.${ext.replace(/^\./, '')}`
}

/** 触发下载；无 document 时 no-op */
export function triggerDownload(filename: string, blob: Blob): void {
  if (typeof document === 'undefined') return
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

export function downloadJSON(filename: string, payload: unknown): void {
  triggerDownload(filename, buildJSONBlob(payload))
}

export function downloadText(filename: string, text: string, mime = 'text/plain'): void {
  triggerDownload(filename, buildTextBlob(text, mime))
}
