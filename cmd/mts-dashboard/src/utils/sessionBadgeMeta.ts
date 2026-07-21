/** TopBar 会话 badge 文案：本地剩余 + 服务端 remaining 校准提示 */

import { formatRemaining } from './sessionExpiry.ts'

export interface SessionBadgeMetaInput {
  localLabel: string
  localRemainingMs: number
  urgency: string
  serverRemainingSec: number | null | undefined
  checkedAtMs: number | null | undefined
  nowMs?: number
  /** 偏差超过该秒数则提示（默认 30s） */
  skewThresholdSec?: number
  locale?: 'zh' | 'en'
}

export interface SessionBadgeMeta {
  title: string
  hint: string
  serverLabel: string
  skewSec: number | null
  showServerHint: boolean
}

function formatCheckedAt(ms: number, locale: 'zh' | 'en'): string {
  try {
    return new Date(ms).toLocaleTimeString(locale === 'en' ? 'en-US' : 'zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  } catch {
    return String(ms)
  }
}

/** 构建会话 badge 的 title / 副文案（warn/critical 时附服务端 remaining） */
export function buildSessionBadgeMeta(input: SessionBadgeMetaInput): SessionBadgeMeta {
  const locale = input.locale === 'en' ? 'en' : 'zh'
  const threshold = input.skewThresholdSec ?? 30
  const now = input.nowMs ?? Date.now()
  const base =
    locale === 'en'
      ? `Session remaining ${input.localLabel || '—'}`
      : `会话剩余 ${input.localLabel || '—'}`

  const serverSec = input.serverRemainingSec
  const hasServer = typeof serverSec === 'number' && Number.isFinite(serverSec)
  if (!hasServer) {
    return {
      title: base,
      hint: '',
      serverLabel: '',
      skewSec: null,
      showServerHint: false,
    }
  }

  const serverMs = Math.max(0, Math.floor(serverSec)) * 1000
  const serverLabel = formatRemaining(serverMs)
  const localSec = Math.max(0, Math.ceil(input.localRemainingMs / 1000))
  const skewSec = localSec - Math.max(0, Math.floor(serverSec))
  const urgent = input.urgency === 'warn' || input.urgency === 'critical' || input.urgency === 'expired'
  const skewAbs = Math.abs(skewSec)
  const showSkew = skewAbs >= threshold
  const checked =
    typeof input.checkedAtMs === 'number' && Number.isFinite(input.checkedAtMs)
      ? formatCheckedAt(input.checkedAtMs, locale)
      : ''

  let title = base
  if (locale === 'en') {
    title = `${base}; server ${serverLabel}`
    if (checked) title += ` (verified ${checked})`
    if (showSkew) title += `; skew ${skewSec}s`
  } else {
    title = `${base}；服务端 ${serverLabel}`
    if (checked) title += `（校验 ${checked}）`
    if (showSkew) title += `；偏差 ${skewSec}s`
  }

  const showServerHint = urgent || showSkew
  let hint = ''
  if (showServerHint) {
    hint =
      locale === 'en'
        ? `srv ${serverLabel}${showSkew ? ` Δ${skewSec}s` : ''}`
        : `端 ${serverLabel}${showSkew ? ` Δ${skewSec}s` : ''}`
  }

  // silence unused now (reserved for future staleness)
  void now

  return { title, hint, serverLabel, skewSec, showServerHint }
}
