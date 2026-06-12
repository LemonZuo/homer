import { Loader2 } from 'lucide-react'

import type { DiskHealth, SeriesPoint } from '../types'
import { buildCoreLines, buildDiskLines, buildDiskUsageLines } from './seriesBuilders'
import { MiniChart } from './MiniChart'
import { MultiLineChart } from './MultiLineChart'
import type { LineSeries, MetricKey, MiniPoint } from './types'

export function EsxiSeriesChart({
  series,
  loading,
  metric,
  disks,
}: {
  series: SeriesPoint[] | null
  loading: boolean
  metric: MetricKey
  disks?: DiskHealth[]
}) {
  if (loading) {
    return (
      <div className="flex h-32 items-center justify-center text-[12px] text-muted-foreground">
        <Loader2 className="mr-1.5 h-3 w-3 animate-spin" />
        加载中
      </div>
    )
  }
  if (!series || series.length === 0) {
    return (
      <div className="flex h-32 items-center justify-center text-[12px] text-muted-foreground">
        暂无历史数据
      </div>
    )
  }
  const ts = series.map((p) => new Date(p.bucket_start).getTime())

  // 多线场景:每核 / 每盘各画一条线。
  if (metric === 'cpu_cores' || metric === 'disk_per_disk' || metric === 'disk_usage') {
    let lines: LineSeries[]
    let unit = '°C'
    let format: (v: number) => string = (v) => v.toFixed(0)
    let missingLabel = '每核'
    if (metric === 'cpu_cores') {
      lines = buildCoreLines(series, ts)
    } else if (metric === 'disk_per_disk') {
      lines = buildDiskLines(series, ts, disks)
      missingLabel = '每盘'
    } else {
      lines = buildDiskUsageLines(series, ts, disks)
      unit = 'GiB'
      format = (v) => (v >= 100 ? v.toFixed(0) : v.toFixed(1))
      missingLabel = '每盘已用'
    }
    if (lines.length === 0) {
      return (
        <div className="flex h-32 items-center justify-center text-[12px] text-muted-foreground">
          暂无{missingLabel}明细
        </div>
      )
    }
    return <MultiLineChart lines={lines} unit={unit} yMin={0} format={format} />
  }

  let data: MiniPoint[]
  let unit: string
  let stroke: string
  let yMin: number | undefined
  let yMax: number | undefined
  let format: (v: number) => string
  switch (metric) {
    case 'cpu_usage':
      data = series.map((p, i) => ({
        t: ts[i],
        v: p.cpu_usage_percent < 0 ? null : p.cpu_usage_percent,
      }))
      unit = '%'
      stroke = 'rgb(79 137 192)'
      yMin = 0
      yMax = 100
      format = (v) => v.toFixed(0)
      break
    case 'memory_used':
      data = series.map((p, i) => ({
        t: ts[i],
        v: p.memory_used_bytes < 0 ? null : p.memory_used_bytes / 1024 ** 3,
      }))
      unit = 'GiB'
      stroke = 'rgb(20 184 166)'
      yMin = 0
      yMax = series.reduce((max, p) => Math.max(max, p.memory_total_bytes ?? 0), 0) / 1024 ** 3
      if (yMax <= 0) yMax = undefined
      format = (v) => (v >= 100 ? v.toFixed(0) : v.toFixed(1))
      break
    case 'vm_on':
      data = series.map((p, i) => ({ t: ts[i], v: p.vm_powered_on < 0 ? null : p.vm_powered_on }))
      unit = ''
      stroke = 'rgb(16 185 129)'
      yMin = 0
      format = (v) => v.toFixed(0)
      break
    case 'mce':
      data = series.map((p, i) => ({
        t: ts[i],
        v: (p.mce_corrected_total ?? 0) + (p.mce_uncorrected_total ?? 0),
      }))
      unit = ''
      stroke = 'rgb(244 63 94)'
      yMin = 0
      format = (v) => v.toFixed(0)
      break
  }
  return <MiniChart data={data} unit={unit} stroke={stroke} yMin={yMin} yMax={yMax} format={format} />
}
