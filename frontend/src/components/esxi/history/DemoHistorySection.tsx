import { useMemo, useState } from 'react'

import { cn } from '../../../lib/utils'
import type { SeriesPoint, Snapshot } from '../types'
import { METRIC_OPTIONS, RANGE_OPTIONS } from './constants'
import { EsxiSeriesChart } from './EsxiSeriesChart'
import type { MetricKey } from './types'

// DemoHistorySection 与 HistorySection 视觉等价，但 series 完全由当前 host 静态字段
// 反推 + 伪随机抖动出来，不打后端。供演示模式使用。
export function DemoHistorySection({ host }: { host: Snapshot }) {
  const [range, setRange] = useState('24h')
  const [metric, setMetric] = useState<MetricKey>('cpu_cores')
  const series = useMemo(() => buildMockSeries(host, range), [host, range])

  return (
    <div className="rounded-md border border-border/60 bg-muted/30 p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap gap-1">
          {METRIC_OPTIONS.map((o) => (
            <button
              key={o.value}
              type="button"
              onClick={() => setMetric(o.value)}
              className={cn(
                'rounded-md border px-2 py-0.5 text-[11px] transition-colors',
                metric === o.value
                  ? 'border-[#4f89c0]/60 bg-[#4f89c0]/15 text-[#3d6e9d] dark:text-[#9bc1e0]'
                  : 'border-border bg-background text-muted-foreground hover:border-border/80 hover:text-foreground',
              )}
            >
              {o.label}
            </button>
          ))}
        </div>
        <div className="flex flex-wrap gap-1">
          {RANGE_OPTIONS.map((o) => (
            <button
              key={o.value}
              type="button"
              onClick={() => setRange(o.value)}
              className={cn(
                'rounded-md border px-2 py-0.5 text-[11px] transition-colors',
                range === o.value
                  ? 'border-[#4f89c0]/60 bg-[#4f89c0]/15 text-[#3d6e9d] dark:text-[#9bc1e0]'
                  : 'border-border bg-background text-muted-foreground hover:border-border/80 hover:text-foreground',
              )}
            >
              {o.label}
            </button>
          ))}
        </div>
      </div>
      <EsxiSeriesChart series={series} loading={false} metric={metric} disks={host.disk_health} />
    </div>
  )
}

const RANGE_BUCKETS: Record<string, { points: number; stepMs: number }> = {
  '1h': { points: 60, stepMs: 60_000 },
  '6h': { points: 72, stepMs: 5 * 60_000 },
  '12h': { points: 72, stepMs: 10 * 60_000 },
  '24h': { points: 96, stepMs: 15 * 60_000 },
  '3d': { points: 72, stepMs: 60 * 60_000 },
  '7d': { points: 84, stepMs: 2 * 60 * 60_000 },
}

function buildMockSeries(host: Snapshot, range: string): SeriesPoint[] {
  const cfg = RANGE_BUCKETS[range] ?? RANGE_BUCKETS['24h']
  const now = Date.now()
  const startTime = now - cfg.points * cfg.stepMs

  // 以 host_id + range 作种子，保证同 host 同 range 抖动稳定。
  const seed = Math.abs(host.host_id || 1) * 1000 + range.length * 13
  const rand = mulberry32(seed)

  const cpuCores = host.cpu_temperature?.cores ?? []
  const disks = host.disk_health ?? []
  const memTotal = host.memory?.mem_total_bytes ?? 0
  const memUsedBase = host.runtime_usage?.memory_used_bytes ?? memTotal * 0.5
  const cpuUsageBase = host.runtime_usage?.cpu_usage_percent ?? 30
  const vmOnBase = (host.vms ?? []).filter((v) => v.state === 'powered_on').length
  const mceBase = host.mce_health?.corrected_total ?? 0

  const series: SeriesPoint[] = []
  let mceAccum = Math.max(0, mceBase - Math.floor(rand() * 5))

  for (let i = 0; i < cfg.points; i++) {
    const t = new Date(startTime + i * cfg.stepMs).toISOString()
    const phase = (i / cfg.points) * Math.PI * 4 // 两个完整周期
    const wave = Math.sin(phase) * 0.5 + (rand() - 0.5) * 0.3

    const cores = cpuCores.map((c) => {
      const v = c.temp_c + wave * 6 + (rand() - 0.5) * 3
      return { id: c.id, temp_c: Math.max(30, +v.toFixed(1)) }
    })

    const disksPoint = disks.map((d) => {
      const v = d.temp_c + wave * 4 + (rand() - 0.5) * 2.5
      return { device: d.device, temp_c: Math.max(20, +v.toFixed(1)) }
    })

    // 磁盘使用量：随时间线性增长 +/- 微抖动，越早的 bucket 已用越小。
    const ago = cfg.points - i
    const diskUsage = disks
      .filter((d) => (d.capacity_bytes ?? 0) > 0)
      .map((d) => {
        const cap = d.capacity_bytes ?? 0
        const usedNow = d.used_bytes ?? 0
        // 假设每天涨 0.1% 容量
        const dailyGrowth = cap * 0.001
        const stepFrac = (ago * cfg.stepMs) / 86_400_000
        const used = Math.max(0, usedNow - dailyGrowth * stepFrac + (rand() - 0.5) * cap * 0.001)
        return { device: d.device, used_bytes: Math.round(used), capacity_bytes: cap }
      })

    const cpuUsagePct = clamp(cpuUsageBase + wave * 15 + (rand() - 0.5) * 8, 0, 100)
    const memUsed = clamp(memUsedBase * (1 + wave * 0.08 + (rand() - 0.5) * 0.04), 0, memTotal || memUsedBase * 2)
    const memUsedPct = memTotal > 0 ? (memUsed / memTotal) * 100 : 0

    // VM 数量大多稳定，偶尔少 1（关机重启等）。
    let vmOn = vmOnBase
    if (vmOnBase > 0 && rand() > 0.94) vmOn = Math.max(0, vmOnBase - 1)

    // MCE 计数：以低频累加。
    if (rand() > 0.96) mceAccum++

    series.push({
      bucket_start: t,
      cpu_max_c: cores.length ? Math.max(...cores.map((c) => c.temp_c)) : 0,
      cpu_avg_c: cores.length ? +(cores.reduce((s, c) => s + c.temp_c, 0) / cores.length).toFixed(1) : 0,
      disk_max_c: disksPoint.length ? Math.max(...disksPoint.map((d) => d.temp_c)) : 0,
      cpu_usage_percent: +cpuUsagePct.toFixed(1),
      memory_used_bytes: Math.round(memUsed),
      memory_total_bytes: memTotal,
      memory_usage_percent: +memUsedPct.toFixed(1),
      mce_corrected_total: mceAccum,
      mce_uncorrected_total: 0,
      vm_powered_on: vmOn,
      cpu_cores: cores,
      disks: disksPoint,
      disk_usage: diskUsage,
    })
  }

  return series
}

function clamp(v: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, v))
}

// mulberry32 是一个轻量的可复现 PRNG，适合演示用。
function mulberry32(seed: number) {
  let s = seed >>> 0
  return function (): number {
    s = (s + 0x6d2b79f5) >>> 0
    let t = s
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}
