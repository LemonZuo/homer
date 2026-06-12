import { useCallback, useEffect, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { api } from '../../../api'
import { cn } from '../../../lib/utils'
import { extractErr, shortDeviceLabel } from '../format'
import type { DiskHealth, SeriesPoint } from '../types'

// 多线曲线的颜色色卡(色相循环);超过 10 条会循环复用。
const LINE_COLORS = [
  'rgb(168 85 247)',
  'rgb(14 165 233)',
  'rgb(16 185 129)',
  'rgb(244 63 94)',
  'rgb(245 158 11)',
  'rgb(99 102 241)',
  'rgb(20 184 166)',
  'rgb(236 72 153)',
  'rgb(132 204 22)',
  'rgb(249 115 22)',
]

type MetricKey =
  | 'cpu_cores'
  | 'disk_per_disk'
  | 'cpu_usage'
  | 'memory_used'
  | 'disk_usage'
  | 'vm_on'
  | 'mce'

const METRIC_OPTIONS: { value: MetricKey; label: string }[] = [
  { value: 'cpu_cores', label: 'CPU 温度' },
  { value: 'disk_per_disk', label: '磁盘温度' },
  { value: 'cpu_usage', label: 'CPU 使用量' },
  { value: 'memory_used', label: '内存使用量' },
  { value: 'disk_usage', label: '磁盘使用量' },
  { value: 'vm_on', label: '运行 VM' },
  { value: 'mce', label: 'MCE 累计' },
]

const RANGE_OPTIONS = [
  { value: '1h', label: '1 小时' },
  { value: '6h', label: '6 小时' },
  { value: '24h', label: '24 小时' },
  { value: '3d', label: '3 天' },
  { value: '7d', label: '7 天' },
]

interface MiniPoint {
  t: number
  v: number | null
}

interface LineSeries {
  id: string
  label: string
  color: string
  points: MiniPoint[]
}

export function HistorySection({
  hostKind,
  hostID,
  disks,
}: {
  hostKind: string
  hostID: number
  disks?: DiskHealth[]
}) {
  const [range, setRange] = useState('24h')
  const [metric, setMetric] = useState<MetricKey>('cpu_cores')
  const [series, setSeries] = useState<SeriesPoint[] | null>(null)
  const [loading, setLoading] = useState(false)

  const load = useCallback(
    async (r: string) => {
      setLoading(true)
      try {
        const { data } = await api.get('/esxi/series', {
          params: { host_kind: hostKind, host_id: hostID, range: r },
        })
        setSeries(data?.data ?? [])
      } catch (e) {
        toast.error(extractErr(e, '加载历史失败'))
      } finally {
        setLoading(false)
      }
    },
    [hostKind, hostID],
  )

  useEffect(() => {
    queueMicrotask(() => {
      void load(range)
    })
  }, [load, range])

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
      <EsxiSeriesChart series={series} loading={loading} metric={metric} disks={disks} />
    </div>
  )
}

function EsxiSeriesChart({
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

// buildCoreLines 把按时间桶的 cpu_cores 明细转成"按核 id 分组"的多条曲线。
// 出现过的核 id 取并集排序;某桶没该核(或整桶 cpu_cores 缺失)落 null,前端跳点不连。
function buildCoreLines(series: SeriesPoint[], ts: number[]): LineSeries[] {
  const idSet = new Set<number>()
  for (const p of series) {
    for (const c of p.cpu_cores ?? []) idSet.add(c.id)
  }
  const ids = [...idSet].sort((a, b) => a - b)
  return ids.map((id, idx) => ({
    id: `core-${id}`,
    label: `核 ${id}`,
    color: LINE_COLORS[idx % LINE_COLORS.length],
    points: series.map((p, i) => {
      const c = (p.cpu_cores ?? []).find((x) => x.id === id)
      return { t: ts[i], v: c && c.temp_c >= 0 ? c.temp_c : null }
    }),
  }))
}

function buildDiskLines(series: SeriesPoint[], ts: number[], disks?: DiskHealth[]): LineSeries[] {
  const devSet = new Set<string>()
  for (const p of series) {
    for (const d of p.disks ?? []) devSet.add(d.device)
  }
  const devs = [...devSet].sort()
  const labelByDevice = new Map((disks ?? []).map((d) => [d.device, d.model || d.type || shortDeviceLabel(d.device)]))
  return devs.map((dev, idx) => ({
    id: `disk-${dev}`,
    label: labelByDevice.get(dev) ?? shortDeviceLabel(dev),
    color: LINE_COLORS[idx % LINE_COLORS.length],
    points: series.map((p, i) => {
      const d = (p.disks ?? []).find((x) => x.device === dev)
      return { t: ts[i], v: d && d.temp_c >= 0 ? d.temp_c : null }
    }),
  }))
}

// buildDiskUsageLines 按 device 维度把 disk_usage(已用字节) 转为 GiB 多线;旧桶或缺失值落 null。
function buildDiskUsageLines(series: SeriesPoint[], ts: number[], disks?: DiskHealth[]): LineSeries[] {
  const devSet = new Set<string>()
  for (const p of series) {
    for (const d of p.disk_usage ?? []) devSet.add(d.device)
  }
  const devs = [...devSet].sort()
  const labelByDevice = new Map((disks ?? []).map((d) => [d.device, d.model || d.type || shortDeviceLabel(d.device)]))
  const GiB = 1024 ** 3
  return devs.map((dev, idx) => ({
    id: `disk-usage-${dev}`,
    label: labelByDevice.get(dev) ?? shortDeviceLabel(dev),
    color: LINE_COLORS[idx % LINE_COLORS.length],
    points: series.map((p, i) => {
      const d = (p.disk_usage ?? []).find((x) => x.device === dev)
      return { t: ts[i], v: d && d.used_bytes > 0 ? d.used_bytes / GiB : null }
    }),
  }))
}

// MultiLineChart 多线版本的 mini chart。顶部 legend 横排,hover 时在 legend 上挂值;
// 主体单 svg 多 path,统一 y 轴(所有线共享 yLo/yHi)。结构刻意贴近 MiniChart,便于以后对齐。
function MultiLineChart({
  lines,
  unit,
  yMin,
  format,
}: {
  lines: LineSeries[]
  unit: string
  yMin?: number
  format: (v: number) => string
}) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(640)
  const [hoverIdx, setHoverIdx] = useState<number | null>(null)

  useEffect(() => {
    const el = wrapRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      for (const e of entries) {
        const w = e.contentRect.width
        if (w > 0) setWidth(w)
      }
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const ts = lines[0]?.points.map((p) => p.t) ?? []
  const allValid: number[] = []
  for (const ln of lines) for (const p of ln.points) if (p.v != null) allValid.push(p.v)
  const allMissing = allValid.length === 0

  const H = 200
  const padL = 40
  const padR = 8
  const padT = 8
  const padB = 18
  const innerW = Math.max(20, width - padL - padR)
  const innerH = H - padT - padB

  const tMin = ts[0] ?? 0
  const tMax = ts[ts.length - 1] ?? 1
  const tSpan = Math.max(1, tMax - tMin)
  const xOf = (t: number) => padL + ((t - tMin) / tSpan) * innerW

  let yLo: number, yHi: number
  if (allMissing) {
    yLo = 0
    yHi = 1
  } else {
    const mn = Math.min(...allValid)
    const mx = Math.max(...allValid)
    if (mn === mx) {
      yLo = mn - 1
      yHi = mx + 1
    } else {
      const pad = (mx - mn) * 0.15
      yLo = mn - pad
      yHi = mx + pad
    }
    if (yMin != null) yLo = yMin
  }
  const ySpan = Math.max(0.001, yHi - yLo)
  const yOf = (v: number) => padT + (1 - (v - yLo) / ySpan) * innerH
  const yTicks = [yLo, (yLo + yHi) / 2, yHi]

  const xTickCount = Math.min(5, ts.length)
  const xTicks = Array.from({ length: xTickCount }, (_, i) =>
    Math.round(tMin + ((tMax - tMin) * i) / Math.max(1, xTickCount - 1)),
  )
  const fmtX = (t: number) => {
    const d = new Date(t)
    const pad = (n: number) => n.toString().padStart(2, '0')
    if (tSpan > 36 * 3600 * 1000) return `${pad(d.getMonth() + 1)}/${pad(d.getDate())}`
    return `${pad(d.getHours())}:${pad(d.getMinutes())}`
  }

  const onMove = (ev: ReactMouseEvent<SVGSVGElement>) => {
    if (ts.length === 0) return
    const svg = ev.currentTarget
    const r = svg.getBoundingClientRect()
    const x = ev.clientX - r.left
    if (x < padL || x > padL + innerW) {
      setHoverIdx(null)
      return
    }
    const ratio = (x - padL) / innerW
    const targetT = tMin + ratio * tSpan
    let bestIdx = 0
    let bestDiff = Infinity
    for (let i = 0; i < ts.length; i++) {
      const diff = Math.abs(ts[i] - targetT)
      if (diff < bestDiff) {
        bestDiff = diff
        bestIdx = i
      }
    }
    setHoverIdx(bestIdx)
  }

  const hoverT = hoverIdx != null ? ts[hoverIdx] : null

  return (
    <div ref={wrapRef} className="relative">
      {/* legend + hover 时间戳 */}
      <div className="mb-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px]">
        {lines.map((ln) => {
          const v = hoverIdx != null ? ln.points[hoverIdx]?.v : null
          return (
            <span key={ln.id} className="inline-flex max-w-[210px] items-center gap-1 tabular-nums sm:max-w-[260px]">
              <span
                className="inline-block h-2 w-2 shrink-0 rounded-full"
                style={{ background: ln.color }}
              />
              <span className="min-w-0 truncate text-muted-foreground" title={ln.label}>{ln.label}</span>
              {v != null && (
                <span className="font-semibold text-foreground">
                  {format(v)}
                  {unit && <span className="ml-0.5 text-muted-foreground">{unit}</span>}
                </span>
              )}
            </span>
          )
        })}
        {hoverT != null && (
          <span className="ml-auto text-muted-foreground tabular-nums">
            {new Date(hoverT).toLocaleString('zh-CN', { hour12: false })}
          </span>
        )}
      </div>
      <svg
        width="100%"
        height={H}
        viewBox={`0 0 ${width} ${H}`}
        preserveAspectRatio="none"
        onMouseMove={onMove}
        onMouseLeave={() => setHoverIdx(null)}
      >
        {yTicks.map((v, i) => (
          <g key={i}>
            <line
              x1={padL}
              x2={width - padR}
              y1={yOf(v)}
              y2={yOf(v)}
              className="stroke-border"
              strokeDasharray="2 4"
            />
            <text
              x={padL - 4}
              y={yOf(v) + 3}
              textAnchor="end"
              fontSize="10"
              className="fill-muted-foreground"
            >
              {format(v)}
            </text>
          </g>
        ))}
        {xTicks.map((t, i) => (
          <text
            key={i}
            x={xOf(t)}
            y={H - 4}
            textAnchor={i === 0 ? 'start' : i === xTicks.length - 1 ? 'end' : 'middle'}
            fontSize="10"
            className="fill-muted-foreground"
          >
            {fmtX(t)}
          </text>
        ))}
        {/* 各条线 path */}
        {!allMissing &&
          lines.map((ln) => {
            let path = ''
            let penUp = true
            for (const d of ln.points) {
              if (d.v == null) {
                penUp = true
                continue
              }
              const cmd = penUp ? 'M' : 'L'
              path += `${cmd}${xOf(d.t).toFixed(1)},${yOf(d.v).toFixed(1)} `
              penUp = false
            }
            return (
              <path
                key={ln.id}
                d={path.trim()}
                fill="none"
                stroke={ln.color}
                strokeWidth={1.5}
                strokeLinecap="round"
                strokeLinejoin="round"
                opacity={0.85}
              />
            )
          })}
        {/* hover 高亮:垂直线 + 各线点 */}
        {hoverIdx != null && hoverT != null && !allMissing && (
          <g>
            <line
              x1={xOf(hoverT)}
              x2={xOf(hoverT)}
              y1={padT}
              y2={padT + innerH}
              className="stroke-border"
              strokeWidth={1}
            />
            {lines.map((ln) => {
              const v = ln.points[hoverIdx]?.v
              if (v == null) return null
              return (
                <circle
                  key={ln.id}
                  cx={xOf(hoverT)}
                  cy={yOf(v)}
                  r={3}
                  fill={ln.color}
                />
              )
            })}
          </g>
        )}
        {allMissing && (
          <text
            x={padL + innerW / 2}
            y={padT + innerH / 2}
            textAnchor="middle"
            fontSize="11"
            className="fill-muted-foreground"
          >
            该指标无数据
          </text>
        )}
      </svg>
    </div>
  )
}

function MiniChart({
  unit,
  data,
  stroke,
  yMin,
  yMax,
  format,
}: {
  unit: string
  data: MiniPoint[]
  stroke: string
  yMin?: number
  yMax?: number
  format: (v: number) => string
}) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(640)
  const [hover, setHover] = useState<{ idx: number } | null>(null)

  useEffect(() => {
    const el = wrapRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      for (const e of entries) {
        const w = e.contentRect.width
        if (w > 0) setWidth(w)
      }
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const validVals = data.map((d) => d.v).filter((v): v is number => v != null)
  const allMissing = validVals.length === 0

  const H = 180
  const padL = 40
  const padR = 8
  const padT = 8
  const padB = 18
  const innerW = Math.max(20, width - padL - padR)
  const innerH = H - padT - padB

  const tMin = data[0].t
  const tMax = data[data.length - 1].t
  const tSpan = Math.max(1, tMax - tMin)
  const xOf = (t: number) => padL + ((t - tMin) / tSpan) * innerW

  let yLo: number, yHi: number
  if (allMissing) {
    yLo = 0
    yHi = 1
  } else {
    const mn = Math.min(...validVals)
    const mx = Math.max(...validVals)
    if (mn === mx) {
      yLo = mn - 1
      yHi = mx + 1
    } else {
      const pad = (mx - mn) * 0.15
      yLo = mn - pad
      yHi = mx + pad
    }
    if (yMin != null) yLo = yMin
    if (yMax != null) yHi = yMax
  }
  const ySpan = Math.max(0.001, yHi - yLo)
  const yOf = (v: number) => padT + (1 - (v - yLo) / ySpan) * innerH
  const yTicks = [yLo, (yLo + yHi) / 2, yHi]

  let path = ''
  let penUp = true
  for (const d of data) {
    if (d.v == null) {
      penUp = true
      continue
    }
    const cmd = penUp ? 'M' : 'L'
    path += `${cmd}${xOf(d.t).toFixed(1)},${yOf(d.v).toFixed(1)} `
    penUp = false
  }

  const xTickCount = Math.min(5, data.length)
  const xTicks = Array.from({ length: xTickCount }, (_, i) =>
    Math.round(tMin + ((tMax - tMin) * i) / Math.max(1, xTickCount - 1)),
  )
  const fmtX = (t: number) => {
    const d = new Date(t)
    const pad = (n: number) => n.toString().padStart(2, '0')
    if (tSpan > 36 * 3600 * 1000) return `${pad(d.getMonth() + 1)}/${pad(d.getDate())}`
    return `${pad(d.getHours())}:${pad(d.getMinutes())}`
  }

  const onMove = (ev: ReactMouseEvent<SVGSVGElement>) => {
    const svg = ev.currentTarget
    const r = svg.getBoundingClientRect()
    const x = ev.clientX - r.left
    if (x < padL || x > padL + innerW) {
      setHover(null)
      return
    }
    const ratio = (x - padL) / innerW
    const targetT = tMin + ratio * tSpan
    let bestIdx = 0
    let bestDiff = Infinity
    for (let i = 0; i < data.length; i++) {
      const diff = Math.abs(data[i].t - targetT)
      if (diff < bestDiff) {
        bestDiff = diff
        bestIdx = i
      }
    }
    setHover({ idx: bestIdx })
  }

  const hoverPoint = hover ? data[hover.idx] : null

  return (
    <div ref={wrapRef} className="relative">
      <div className="mb-1 flex h-4 items-center justify-end text-[11.5px] text-muted-foreground">
        {hoverPoint && hoverPoint.v != null ? (
          <span className="tabular-nums">
            <span className="text-foreground font-semibold">{format(hoverPoint.v)}</span>
            {unit && <span className="ml-0.5">{unit}</span>}
            <span className="ml-2 text-muted-foreground">
              {new Date(hoverPoint.t).toLocaleString('zh-CN', { hour12: false })}
            </span>
          </span>
        ) : (
          <span>{unit}</span>
        )}
      </div>
      <svg
        width="100%"
        height={H}
        viewBox={`0 0 ${width} ${H}`}
        preserveAspectRatio="none"
        onMouseMove={onMove}
        onMouseLeave={() => setHover(null)}
      >
        {yTicks.map((v, i) => (
          <g key={i}>
            <line
              x1={padL}
              x2={width - padR}
              y1={yOf(v)}
              y2={yOf(v)}
              className="stroke-border"
              strokeDasharray="2 4"
            />
            <text
              x={padL - 4}
              y={yOf(v) + 3}
              textAnchor="end"
              fontSize="10"
              className="fill-muted-foreground"
            >
              {format(v)}
            </text>
          </g>
        ))}
        {xTicks.map((t, i) => (
          <text
            key={i}
            x={xOf(t)}
            y={H - 4}
            textAnchor={i === 0 ? 'start' : i === xTicks.length - 1 ? 'end' : 'middle'}
            fontSize="10"
            className="fill-muted-foreground"
          >
            {fmtX(t)}
          </text>
        ))}
        {!allMissing && (
          <path
            d={path.trim()}
            fill="none"
            stroke={stroke}
            strokeWidth={1.75}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        )}
        {hoverPoint && hoverPoint.v != null && (
          <g>
            <line
              x1={xOf(hoverPoint.t)}
              x2={xOf(hoverPoint.t)}
              y1={padT}
              y2={padT + innerH}
              className="stroke-border"
              strokeWidth={1}
            />
            <circle cx={xOf(hoverPoint.t)} cy={yOf(hoverPoint.v)} r={3.5} fill={stroke} />
          </g>
        )}
        {allMissing && (
          <text
            x={padL + innerW / 2}
            y={padT + innerH / 2}
            textAnchor="middle"
            fontSize="11"
            className="fill-muted-foreground"
          >
            该指标无数据
          </text>
        )}
      </svg>
    </div>
  )
}
