import { useEffect, useState } from 'react'

import { BATTERY_TYPE_LABEL } from './constants'

export function fmtRuntime(min: number): string {
  if (min < 0) return '— —'
  if (min < 60) return `${min} min`
  const h = Math.floor(min / 60)
  const m = min % 60
  return `${h}h ${m.toString().padStart(2, '0')}m`
}

export function fmtTime(s: string): string {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`
}

export function fmtNum(v: number, fractionDigits = 0): string {
  if (v == null || v < 0 || !isFinite(v)) return '—'
  return v.toFixed(fractionDigits)
}

// 超过 10 分钟没拿到新一帧采样即视为离线(SSE 推送 + 后端调度通常每轮 < 30s)
const STALE_THRESHOLD_MS = 10 * 60_000

export function useNowTick(intervalMs = 10_000): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])
  return now
}

export function isStaleSample(sampledAt: string, now: number): boolean {
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

export function extractErr(e: unknown, fallback: string): string {
  if (e && typeof e === 'object') {
    const obj = e as { response?: { data?: { error?: string } }; message?: string }
    return obj.response?.data?.error || obj.message || fallback
  }
  return fallback
}

export function fmtBatteryType(s: string): string {
  if (!s) return '—'
  return BATTERY_TYPE_LABEL[s.toLowerCase()] ?? s
}

export function fmtDateTime(s: string): string {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
