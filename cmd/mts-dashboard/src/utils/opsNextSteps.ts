/** 就绪中心/概览：按优先级给出本地下一步建议（不计分、不宣称验收完成） */

import type { LocaleCode } from './localizedText.ts'
import type { ExportPreflightResult } from './exportPreflight.ts'
import {
  assessSignoffCompleteness,
  signoffFieldLabel,
  type SignoffNoteField,
} from './signoffExport.ts'
import type { SignoffNotes } from './readinessState.ts'

export type NextStepActionKey = 'preflightJumpLocal' | 'preflightJumpStorage'

export interface OpsNextStep {
  id: string
  /** 已本地化文案 */
  message: string
  target?: string
  actionKey?: NextStepActionKey
  /** 优先级，数字越小越靠前 */
  priority: number
}

export interface BuildOpsNextStepsInput {
  locale?: LocaleCode
  preflight: ExportPreflightResult
  signoffNotes?: SignoffNotes | null
  /** 最多返回几条（不含 done） */
  limit?: number
}

const priorityById: Record<string, number> = {
  signoff: 10,
  checklist: 20,
  edgeHttps: 30,
  backupSchedule: 40,
  doctor: 50,
  'doctor-ok': 51,
  'doctor-warn': 52,
  deployKit: 60,
  tls: 70,
}

function signoffMessage(locale: LocaleCode, missing: SignoffNoteField[]): string {
  const labels = missing.map((f) => signoffFieldLabel(f, locale)).join(locale === 'en' ? ', ' : '、')
  if (locale === 'en') {
    return `Complete sign-off notes: ${labels}`
  }
  return `补全签核备注：${labels}`
}

function messageForItem(
  locale: LocaleCode,
  id: string,
  fallback: string,
  missing: SignoffNoteField[],
): string {
  if (id === 'signoff' && missing.length) {
    return signoffMessage(locale, missing)
  }
  return fallback
}

/**
 * 从导出预检结果提炼「建议下一步」列表。
 * 仅展示 warn 与部署材料 info；footer 与纯 info 状态不进列表（除非仅剩余 done）。
 */
export function buildOpsNextSteps(input: BuildOpsNextStepsInput): OpsNextStep[] {
  const locale: LocaleCode = input.locale === 'en' ? 'en' : 'zh'
  const limit = Math.max(1, input.limit ?? 4)
  const completeness = assessSignoffCompleteness(input.signoffNotes)

  const candidates: OpsNextStep[] = []
  for (const item of input.preflight.items) {
    if (item.id === 'footer') continue
    if (item.level === 'ok') continue
    // deployKit 的 info 仍给出动作；其余 info（如 tls）优先级靠后仍可展示
    if (item.level === 'info' && item.id !== 'deployKit' && item.id !== 'tls') continue

    const priority = priorityById[item.id] ?? 100
    candidates.push({
      id: item.id,
      message: messageForItem(locale, item.id, item.message, completeness.missing),
      target: item.target,
      actionKey: item.actionKey,
      priority,
    })
  }

  candidates.sort((a, b) => a.priority - b.priority || a.id.localeCompare(b.id))
  const sliced = candidates.slice(0, limit)
  if (sliced.length === 0) {
    return [
      {
        id: 'done',
        message:
          locale === 'en'
            ? 'Local preflight and sign-off look complete (deployment-side acceptance still required)'
            : '本地预检与签核项已齐（仍需部署侧人工验收）',
        priority: 0,
      },
    ]
  }
  return sliced
}
