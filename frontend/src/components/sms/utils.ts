import type { SmsItem } from './types'

export function fmtJSON(v: any) {
  try {
    return JSON.stringify(v, null, 2)
  } catch {
    return String(v)
  }
}

export const textOf = (it: SmsItem) => it.content ?? ''

export const whoOf = (it: SmsItem) => {
  const num = it.number?.trim()
  const name = it.name?.trim()
  if (num && name && name !== 'Unknown Number') return `${name} · ${num}`
  return num || name || '—'
}

export function fmtTime(v: any): string {
  if (v == null || v === '') return ''
  const n = Number(v)
  if (Number.isFinite(n) && n > 0) {
    const ms = n < 1e12 ? n * 1000 : n
    const d = new Date(ms)
    if (!isNaN(d.getTime())) {
      const p = (x: number) => String(x).padStart(2, '0')
      return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
    }
  }
  return String(v)
}

export const timeOf = (it: SmsItem) => fmtTime(it.date)

export const simLabel = (id?: number) => {
  if (id === 0) return 'SIM1'
  if (id === 1) return 'SIM2'
  return null
}

// SmsForwarder 标准信封 {code, msg, data: [...]}，data 即条目数组。
export function extractList(payload: any): SmsItem[] {
  if (!payload) return []
  if (Array.isArray(payload?.data)) return payload.data
  if (Array.isArray(payload)) return payload
  return []
}
