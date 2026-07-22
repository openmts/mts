/** 就绪导出前预检（纯函数；提示缺口，不阻止评分外强制验收） */

import type { LocaleCode } from './localizedText.ts'
import type { SignoffNotes } from './readinessState.ts'
import {
  assessSignoffCompleteness,
  signoffFieldLabel,
  type SignoffNoteField,
} from './signoffExport.ts'

export type PreflightLevel = 'ok' | 'warn' | 'info'

export interface ExportPreflightItem {
  id: string
  level: PreflightLevel
  /** 已本地化消息 */
  message: string
  /** 页内或跨页跳转目标（hash 或 path#hash）；无则不可跳转 */
  target?: string
  /** 跳转按钮文案 key（页面侧 i18n） */
  actionKey?: 'preflightJumpLocal' | 'preflightJumpStorage'
}

export interface ExportPreflightInput {
  locale?: LocaleCode
  requiredChecklistRatio: number
  edgeHttpsRequiredRatio: number
  backupScheduleRequiredRatio: number
  doctorLoaded: boolean
  doctorOk?: boolean
  doctorWarnCount?: number
  httpTlsEnabled?: boolean | null
  /** 服务端管理重操作占用（建议项） */
  adminOpBusy?: boolean | null
  /** 当前管理重操作类型展示名（可选，仅 busy 时用于文案） */
  adminOpKindLabel?: string | null
  signoffNotes?: SignoffNotes | null
  deployKitReviewed?: boolean
  /** 客户端相对服务端时钟偏差（秒）；|skew|>=阈值时 warn */
  clockSkewSeconds?: number | null
  clockSkewWarnAbsSec?: number
  /** 数据面契约是否已加载 */
  dataContractLoaded?: boolean | null
  /** 数据面契约必选能力是否齐全 */
  dataContractComplete?: boolean | null
  /** 缺失能力 id 列表（可选，用于文案） */
  dataContractMissing?: string[] | null
}

export interface ExportPreflightResult {
  items: ExportPreflightItem[]
  warnCount: number
  infoCount: number
  okCount: number
  readyToExport: true
}

const copy = {
  zh: {
    checklistOk: '生产必做清单已齐',
    checklistGap: '生产必做清单未齐（{pct}%）',
    edgeOk: '边缘 HTTPS 必做项已齐',
    edgeGap: '边缘 HTTPS 必做项未齐（{pct}%）',
    backupOk: '备份编排必做项已齐',
    backupGap: '备份编排必做项未齐（{pct}%）',
    doctorOk: 'Doctor 已加载且无告警',
    doctorMissing: 'Doctor 未加载，导出将记录 unavailable',
    doctorWarn: 'Doctor 存在 {n} 条 warn',
    doctorNotOk: 'Doctor 报告 not ok',
    tlsOff: 'HTTP TLS 未启用（边缘 HTTPS 仍需部署侧验收）',
    signoffOk: '三项签核备注已填写',
    signoffGap: '签核备注未齐：缺 {missing}',
    deployReviewed: '部署材料包本地已查阅',
    deployNotReviewed: '部署材料包本地尚未勾选「已查阅」',
    adminBusy: '服务端管理重操作占用中（运维/快照/恢复）；建议空闲后再做验收导出',
    adminBusyWithOp: '服务端管理重操作占用中：{op}；建议空闲后再做验收导出',
    adminIdle: '服务端无管理重操作占用',
    clockSkewOk: '客户端与服务端时钟偏差正常',
    clockSkewWarn: '客户端与服务端时钟偏差较大（{skew}，阈值 {threshold}s）；验收前请核对 NTP',
    clockSkewUnknown: '尚未取得服务端时间样本，无法评估时钟偏差',
    dataContractOk: '数据面契约能力齐全（limits/write/query/stream/delete meta）',
    dataContractMissingLoad: '数据面契约未加载（GET /data/contract）',
    dataContractGap: '数据面契约能力缺失：{missing}',
    footer: '预检不阻止导出；完成项不代表生产人工验收已签字',
  },
  en: {
    checklistOk: 'Required production checklist complete',
    checklistGap: 'Required production checklist incomplete ({pct}%)',
    edgeOk: 'Required edge HTTPS items complete',
    edgeGap: 'Required edge HTTPS items incomplete ({pct}%)',
    backupOk: 'Required backup schedule items complete',
    backupGap: 'Required backup schedule items incomplete ({pct}%)',
    doctorOk: 'Doctor loaded with no warns',
    doctorMissing: 'Doctor not loaded; export will record unavailable',
    doctorWarn: 'Doctor has {n} warn(s)',
    doctorNotOk: 'Doctor reports not ok',
    tlsOff: 'HTTP TLS disabled (edge HTTPS still needs deployment-side acceptance)',
    signoffOk: 'All three sign-off notes filled',
    signoffGap: 'Sign-off notes incomplete: missing {missing}',
    deployReviewed: 'Deployment kit marked reviewed locally',
    deployNotReviewed: 'Deployment kit not marked reviewed locally',
    adminBusy: 'Server admin heavy op busy (ops/snapshot/restore); prefer idle before acceptance export',
    adminBusyWithOp: 'Server admin heavy op busy: {op}; prefer idle before acceptance export',
    adminIdle: 'No server admin heavy op in progress',
    clockSkewOk: 'Client/server clock skew is within tolerance',
    clockSkewWarn: 'Client/server clock skew is large ({skew}, threshold {threshold}s); verify NTP before acceptance',
    clockSkewUnknown: 'No server time sample yet; clock skew not evaluated',
    dataContractOk: 'Data-plane contract features complete (limits/write/query/stream/delete meta)',
    dataContractMissingLoad: 'Data-plane contract not loaded (GET /data/contract)',
    dataContractGap: 'Data-plane contract features missing: {missing}',
    footer: 'Preflight does not block export; done items do not mean production acceptance is signed',
  },
} as const

function pct(ratio: number): number {
  if (!Number.isFinite(ratio) || ratio < 0) return 0
  if (ratio > 1) return 100
  return Math.round(ratio * 100)
}

function fill(template: string, vars: Record<string, string | number>): string {
  return template.replace(/\{(\w+)\}/g, (_, k: string) => String(vars[k] ?? ''))
}


/** 预检项 -> 锚点/路径映射（纯函数） */
export function preflightItemTarget(id: string): { target: string; actionKey: 'preflightJumpLocal' | 'preflightJumpStorage' } | null {
  switch (id) {
    case 'checklist':
      return { target: '#production-checklist', actionKey: 'preflightJumpLocal' }
    case 'edgeHttps':
      return { target: '#edge-https-checklist', actionKey: 'preflightJumpLocal' }
    case 'backupSchedule':
      return { target: '#backup-schedule-checklist', actionKey: 'preflightJumpLocal' }
    case 'doctor':
    case 'doctor-ok':
    case 'doctor-warn':
    case 'tls':
      return { target: '#doctor-panel', actionKey: 'preflightJumpLocal' }
    case 'signoff':
      return { target: '#signoff-notes', actionKey: 'preflightJumpLocal' }
    case 'deployKit':
      return { target: '#deploy-kit', actionKey: 'preflightJumpLocal' }
    // 边缘 HTTPS 细项也可去 Storage 验收区（同页已有清单时优先本地）
    case 'edge-storage':
      return { target: '/storage#edge-https', actionKey: 'preflightJumpStorage' }
    case 'admin-op-busy':
      return { target: '/operations#ops-status-strip', actionKey: 'preflightJumpLocal' }
    case 'clockSkew':
      return { target: '/account#account-session', actionKey: 'preflightJumpLocal' }
    case 'data-contract':
      return { target: '#commercial-handoff-panel', actionKey: 'preflightJumpLocal' }
    default:
      return null
  }
}

function withTarget(item: ExportPreflightItem): ExportPreflightItem {
  const mapped = preflightItemTarget(item.id)
  if (!mapped) return item
  return { ...item, target: mapped.target, actionKey: mapped.actionKey }
}


function pushClockSkewItem(
  items: ExportPreflightItem[],
  t: (typeof copy)[LocaleCode],
  skew: number | null | undefined,
  warnAbsSec?: number,
): void {
  const threshold =
    typeof warnAbsSec === 'number' && Number.isFinite(warnAbsSec) && warnAbsSec > 0
      ? Math.floor(warnAbsSec)
      : 30
  if (typeof skew === 'number' && Number.isFinite(skew)) {
    const abs = Math.abs(skew)
    const sign = skew > 0 ? '+' : ''
    const label = `${sign}${Math.round(skew)}s`
    if (abs >= threshold) {
      items.push({
        id: 'clockSkew',
        level: 'warn',
        message: fill(t.clockSkewWarn, { skew: label, threshold }),
      })
    } else {
      items.push({ id: 'clockSkew', level: 'ok', message: t.clockSkewOk })
    }
    return
  }
  if (skew === null) {
    items.push({ id: 'clockSkew', level: 'info', message: t.clockSkewUnknown })
  }
}


function pushDataContractItem(
  items: ExportPreflightItem[],
  t: (typeof copy)[LocaleCode],
  loaded?: boolean | null,
  complete?: boolean | null,
  missing?: string[] | null,
): void {
  if (loaded === true) {
    if (complete === true) {
      items.push({ id: 'data-contract', level: 'ok', message: t.dataContractOk })
      return
    }
    const miss = Array.isArray(missing) && missing.length ? missing.join(',') : 'unknown'
    items.push({
      id: 'data-contract',
      level: 'warn',
      message: fill(t.dataContractGap, { missing: miss }),
    })
    return
  }
  if (loaded === false) {
    items.push({ id: 'data-contract', level: 'warn', message: t.dataContractMissingLoad })
  }
}

export function buildExportPreflight(input: ExportPreflightInput): ExportPreflightResult {
  const locale: LocaleCode = input.locale === 'en' ? 'en' : 'zh'
  const t = copy[locale]
  const items: ExportPreflightItem[] = []

  const reqPct = pct(input.requiredChecklistRatio)
  items.push({
    id: 'checklist',
    level: reqPct >= 100 ? 'ok' : 'warn',
    message: reqPct >= 100 ? t.checklistOk : fill(t.checklistGap, { pct: reqPct }),
  })

  const edgePct = pct(input.edgeHttpsRequiredRatio)
  items.push({
    id: 'edgeHttps',
    level: edgePct >= 100 ? 'ok' : 'warn',
    message: edgePct >= 100 ? t.edgeOk : fill(t.edgeGap, { pct: edgePct }),
  })

  const backupPct = pct(input.backupScheduleRequiredRatio)
  items.push({
    id: 'backupSchedule',
    level: backupPct >= 100 ? 'ok' : 'warn',
    message: backupPct >= 100 ? t.backupOk : fill(t.backupGap, { pct: backupPct }),
  })

  if (!input.doctorLoaded) {
    items.push({ id: 'doctor', level: 'warn', message: t.doctorMissing })
  } else {
    if (input.doctorOk === false) {
      items.push({ id: 'doctor-ok', level: 'warn', message: t.doctorNotOk })
    }
    const warns = Math.max(0, input.doctorWarnCount ?? 0)
    if (warns > 0) {
      items.push({ id: 'doctor-warn', level: 'warn', message: fill(t.doctorWarn, { n: warns }) })
    } else if (input.doctorOk !== false) {
      items.push({ id: 'doctor', level: 'ok', message: t.doctorOk })
    }
    if (input.httpTlsEnabled === false) {
      items.push({ id: 'tls', level: 'info', message: t.tlsOff })
    }
  }

  const sc = assessSignoffCompleteness(input.signoffNotes)
  if (sc.complete) {
    items.push({ id: 'signoff', level: 'ok', message: t.signoffOk })
  } else {
    const missing = sc.missing
      .map((f: SignoffNoteField) => signoffFieldLabel(f, locale))
      .join(locale === 'en' ? ', ' : '、')
    items.push({
      id: 'signoff',
      level: 'warn',
      message: fill(t.signoffGap, { missing }),
    })
  }

  items.push({
    id: 'deployKit',
    level: input.deployKitReviewed ? 'ok' : 'info',
    message: input.deployKitReviewed ? t.deployReviewed : t.deployNotReviewed,
  })

  if (input.adminOpBusy) {
    const opLabel = (input.adminOpKindLabel || '').trim()
    items.push({
      id: 'admin-op-busy',
      level: 'info',
      message: opLabel ? fill(t.adminBusyWithOp, { op: opLabel }) : t.adminBusy,
      target: '/operations#ops-status-strip',
      actionKey: 'preflightJumpLocal',
    })
  } else if (input.adminOpBusy === false) {
    items.push({ id: 'admin-op-busy', level: 'ok', message: t.adminIdle })
  }

  pushClockSkewItem(items, t, input.clockSkewSeconds, input.clockSkewWarnAbsSec)
  pushDataContractItem(
    items,
    t,
    input.dataContractLoaded,
    input.dataContractComplete,
    input.dataContractMissing,
  )

  items.push({ id: 'footer', level: 'info', message: t.footer })

  const enriched = items.map(withTarget)
  return {
    items: enriched,
    warnCount: enriched.filter((i) => i.level === 'warn').length,
    infoCount: enriched.filter((i) => i.level === 'info').length,
    okCount: enriched.filter((i) => i.level === 'ok').length,
    readyToExport: true,
  }
}

/** 预检纯文本摘要，便于复制交接 */
export function formatExportPreflightText(
  result: ExportPreflightResult,
  locale: LocaleCode = 'zh',
): string {
  const head =
    locale === 'en'
      ? [
          'MTS export preflight',
          `ok=${result.okCount} warn=${result.warnCount} info=${result.infoCount}`,
          'Preflight does not block export or complete acceptance.',
          '',
        ]
      : [
          'MTS 导出前预检',
          `ok=${result.okCount} warn=${result.warnCount} info=${result.infoCount}`,
          '预检不阻止导出，不代表生产验收完成。',
          '',
        ]
  const lines = [...head]
  for (const item of result.items) {
    const jump = item.target ? ` -> ${item.target}` : ''
    lines.push(`[${item.level}] ${item.id}: ${item.message}${jump}`)
  }
  return lines.join('\n')
}
