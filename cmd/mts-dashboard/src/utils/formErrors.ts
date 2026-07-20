/** 表单/查询/写入校验文案（locale 表注入，便于单测） */

import { formatMessage } from './formatMessage.ts'

export type FormErrorTable = Record<string, string>

export type FormErrorT = (key: string, vars?: Record<string, string | number>) => string

export function makeFormErrorT(table: FormErrorTable): FormErrorT {
  return (key, vars = {}) => formatMessage(table[key] ?? key, vars)
}
