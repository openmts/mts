/** 简单占位符替换：{name} */

export function formatMessage(
  template: string,
  vars: Record<string, string | number | undefined | null> = {},
): string {
  return String(template || '').replace(/\{(\w+)\}/g, (_, key: string) => {
    const v = vars[key]
    return v == null ? '' : String(v)
  })
}
