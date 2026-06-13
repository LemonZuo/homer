export function fmtTime(v?: string): string {
  if (!v) return '—'
  const d = new Date(v)
  if (isNaN(d.getTime())) return '—'
  const p = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

export function fmtDuration(a: string, b: string): string {
  const ms = new Date(b).getTime() - new Date(a).getTime()
  if (!Number.isFinite(ms) || ms < 0) return ''
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

export const triggerLabel = (trigger: string) => (trigger === 'manual' ? '手动' : '定时')

const WEEKDAYS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

export function describeCron(spec: string): string {
  const f = spec.trim().split(/\s+/)
  if (f.length !== 6) return ''
  const [sec, min, hour, dom, mon, dow] = f
  const num = (v: string) => (/^\d+$/.test(v) ? Number(v) : null)
  const s = num(sec)
  const m = num(min)
  const h = num(hour)
  if (s == null || m == null || h == null) return ''
  const p = (x: number) => String(x).padStart(2, '0')
  const at = `${p(h)}:${p(m)}:${p(s)}`

  if (dom === '*' && mon === '*' && dow === '*') return `每天 ${at}`
  if (dom === '*' && mon === '*') {
    const d = num(dow)
    if (d != null && d >= 0 && d <= 6) return `每${WEEKDAYS[d]} ${at}`
  }
  if (mon === '*' && dow === '*') {
    const d = num(dom)
    if (d != null) return `每月 ${d} 日 ${at}`
  }
  return ''
}
