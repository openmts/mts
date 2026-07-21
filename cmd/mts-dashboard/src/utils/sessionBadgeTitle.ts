/** TopBar 会话 badge title / aria 组装 */

import { formatMessage } from './formatMessage.ts'
import { buildSessionBadgeMeta, type SessionBadgeMeta } from './sessionBadgeMeta.ts'

export function sessionBadgeTitleText(hint: string, metaTitle: string): string {
  if (!metaTitle) return hint
  return `${hint} · ${metaTitle}`
}

export function sessionBadgeAriaText(
  expiryLabel: string,
  localLeft: string,
  unknownLabel: string,
  ariaServerTemplate: string,
  meta: SessionBadgeMeta,
): string {
  const left = localLeft || unknownLabel
  if (!meta.showServerHint || !meta.serverLabel) {
    return `${expiryLabel}: ${left}`
  }
  return formatMessage(ariaServerTemplate, { local: left, server: meta.serverLabel })
}

export { buildSessionBadgeMeta }
