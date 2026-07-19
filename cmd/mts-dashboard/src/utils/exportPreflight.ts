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
  signoffNotes?: SignoffNotes | null
  deployKitReviewed?: boolean
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

  items.push({ id: 'footer', level: 'info', message: t.footer })

  return {
    items,
    warnCount: items.filter((i) => i.level === 'warn').length,
    infoCount: items.filter((i) => i.level === 'info').length,
    okCount: items.filter((i) => i.level === 'ok').length,
    readyToExport: true,
  }
}
