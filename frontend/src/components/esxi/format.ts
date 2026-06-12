import { useEffect, useState } from 'react'

import type { DiskHealth } from './types'

const STALE_THRESHOLD_MS = 30 * 60_000

export function useNowTick(intervalMs = 10_000): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])
  return now
}

export function isStaleSample(sampledAt: string | undefined, now: number): boolean {
  if (!sampledAt) return true
  const t = new Date(sampledAt).getTime()
  if (!isFinite(t)) return true
  return now - t > STALE_THRESHOLD_MS
}

export function fmtStaleAge(ms: number): string {
  if (ms < 0) return ''
  const sec = Math.floor(ms / 1000)
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m`
  const h = Math.floor(min / 60)
  return `${h}h ${min % 60}m`
}

export function fmtDateTime(s: string | undefined): string {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function fmtBytes(n: number): string {
  if (!isFinite(n) || n <= 0) return '—'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

export function fmtBytesWithZero(n: number): string {
  if (!isFinite(n) || n < 0) return '—'
  if (n === 0) return '0 B'
  return fmtBytes(n)
}

export function fmtKB(n: number): string {
  if (!isFinite(n) || n <= 0) return '—'
  return fmtBytes(n * 1024)
}

export function fmtFreq(mhz: number): string {
  if (!isFinite(mhz) || mhz <= 0) return '—'
  if (mhz >= 1000) return `${(mhz / 1000).toFixed(2)} GHz`
  return `${mhz} MHz`
}

export function fmtUptime(sec: number): string {
  if (sec < 0) return '—'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

export function fmtBitrate(mbps: number): string {
  if (mbps < 0) return '—'
  if (mbps >= 1000) return `${(mbps / 1000).toFixed(mbps % 1000 === 0 ? 0 : 1)} Gbps`
  return `${mbps} Mbps`
}

export function extractErr(e: unknown, fallback: string): string {
  if (e && typeof e === 'object') {
    const obj = e as { response?: { data?: { error?: string } }; message?: string }
    return obj.response?.data?.error || obj.message || fallback
  }
  return fallback
}

export function tempTone(temp: number, headroom: number): { text: string; bar: string } {
  if (temp < 0) return { text: 'text-muted-foreground', bar: 'bg-muted-foreground/40' }
  if (headroom >= 0 && headroom < 15) return { text: 'text-rose-600 dark:text-rose-400', bar: 'bg-rose-500' }
  if (headroom >= 0 && headroom < 30) return { text: 'text-amber-600 dark:text-amber-400', bar: 'bg-amber-500' }
  if (temp >= 85) return { text: 'text-rose-600 dark:text-rose-400', bar: 'bg-rose-500' }
  if (temp >= 70) return { text: 'text-amber-600 dark:text-amber-400', bar: 'bg-amber-500' }
  return { text: 'text-emerald-600 dark:text-emerald-400', bar: 'bg-emerald-500' }
}

export function diskStatusPill(status: string) {
  switch (status) {
    case 'ok':
      return { cls: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300', label: '正常' }
    case 'warning':
      return { cls: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300', label: '偏高' }
    case 'critical':
      return { cls: 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300', label: '过热' }
    default:
      return { cls: 'border-border bg-muted text-muted-foreground', label: '未知' }
  }
}

export function diskUsageInfo(d: DiskHealth) {
  const capacity = d.capacity_bytes ?? 0
  const used = d.used_bytes ?? -1
  const free = d.free_bytes ?? -1
  const usageKnown = used >= 0 && (used > 0 || free > 0)
  const total = capacity > 0 ? capacity : usageKnown ? used + Math.max(0, free) : 0
  const pct = usageKnown && total > 0 ? Math.max(0, Math.min(100, (used / total) * 100)) : null
  const label = usageKnown && total > 0
    ? `${fmtBytesWithZero(used)} / ${fmtBytes(total)}`
    : capacity > 0
      ? `总 ${fmtBytes(capacity)}`
      : '容量 —'
  return { pct, label, capacity, used, free }
}

export function vmStatePill(state: string) {
  switch (state) {
    case 'powered_on':
      return { cls: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300', label: '运行中', dot: 'bg-emerald-500' }
    case 'powered_off':
      return { cls: 'border-border bg-muted text-muted-foreground', label: '已关机', dot: 'bg-muted-foreground/60' }
    case 'suspended':
      return { cls: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300', label: '挂起', dot: 'bg-amber-500' }
    default:
      return { cls: 'border-border bg-muted text-muted-foreground', label: state || '未知', dot: 'bg-muted-foreground/60' }
  }
}

export function shortDeviceLabel(dev: string): string {
  if (dev.length <= 14) return dev
  return '…' + dev.slice(-12)
}
