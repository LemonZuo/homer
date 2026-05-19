export function fmtDate(s?: string | null) {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

export function fmtDateTime(s?: string | null) {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

export function daysUntil(s?: string | null): number | null {
  if (!s) return null
  const d = new Date(s)
  if (isNaN(d.getTime())) return null
  return Math.ceil((d.getTime() - Date.now()) / 86400000)
}

export const TASK_PAGE_SIZES = [5, 10, 20, 50, 100]
export const TASK_PAGE_SIZE_KEY = 'acme.taskPageSize'

export function readTaskPageSize(): number {
  const v = Number(localStorage.getItem(TASK_PAGE_SIZE_KEY))
  return TASK_PAGE_SIZES.includes(v) ? v : TASK_PAGE_SIZES[0]
}
