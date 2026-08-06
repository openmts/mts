const LEADING_CONTROL = /^[\u0000-\u001f\u007f]/
const FORMULA_AFTER_CONTROLS = /^[\u0000-\u001f\u007f]*[=+\-@]/

export function escapeCSVCell(value: unknown): string {
  let cell = String(value ?? '')
  if (LEADING_CONTROL.test(cell) || FORMULA_AFTER_CONTROLS.test(cell)) {
    cell = "'" + cell
  }
  if (/[",\n\r]/.test(cell)) {
    return '"' + cell.replace(/"/g, '""') + '"'
  }
  return cell
}
