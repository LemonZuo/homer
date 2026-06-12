import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { api } from '../../../api'
import { cn } from '../../../lib/utils'
import { METRIC_OPTIONS, RANGE_OPTIONS, type MetricKey } from '../constants'
import { extractErr } from '../format'
import type { PowerSource, SeriesPoint } from '../types'

interface MiniPoint {
  t: number
  v: number | null
  ps: PowerSource
}

export function SeriesChart({
  hostKind,
  hostID,
  upsName,
  expanded,
}: {
  hostKind: string
  hostID: number
  upsName: string
  expanded: boolean
}) {
  const [range, setRange] = useState('24h')
  const [metric, setMetric] = useState<MetricKey>('inputV')
  const [points, setPoints] = useState<SeriesPoint[] | null>(null)
  const [loading, setLoading] = useState(false)

  const load = useCallback(
    async (r: string) => {
      setLoading(true)
      try {
        const { data } = await api.get('/ups/series', {
          params: {
            host_kind: hostKind,
            host_id: hostID,
            ups_name: upsName,
            range: r,
          },
        })
        setPoints(data?.data ?? [])
      } catch (e) {
        toast.error(extractErr(e, '加载历史失败'))
      } finally {
        setLoading(false)
      }
    },
    [hostKind, hostID, upsName],
  )

  useEffect(() => {
    if (!expanded) return
    queueMicrotask(() => {
      void load(range)
    })
  }, [expanded, range, load])

  if (!expanded) return null

  return (
    <div className="mt-3 rounded-md border border-border/60 bg-muted/30 p-3">
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
                  ? 'border-teal-500/60 bg-teal-500/10 text-teal-700 dark:text-teal-300'
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
                  ? 'border-teal-500/60 bg-teal-500/10 text-teal-700 dark:text-teal-300'
                  : 'border-border bg-background text-muted-foreground hover:border-border/80 hover:text-foreground',
              )}
            >
              {o.label}
            </button>
          ))}
        </div>
      </div>
      <SeriesPlot points={points} loading={loading} metric={metric} />
    </div>
  )
}

function SeriesPlot({
  points,
  loading,
  metric,
}: {
  points: SeriesPoint[] | null
  loading: boolean
  metric: MetricKey
}) {
  if (loading) {
    return (
      <div className="flex h-32 items-center justify-center text-[12px] text-muted-foreground">
        <Loader2 className="mr-1.5 h-3 w-3 animate-spin" />
        加载中
      </div>
    )
  }
  if (!points || points.length === 0) {
    return (
      <div className="flex h-32 items-center justify-center text-[12px] text-muted-foreground">
        暂无历史数据
      </div>
    )
  }
  const ts = points.map((p) => new Date(p.bucket_start).getTime())
  const psArr = points.map((p) => p.power_source)
  let data: MiniPoint[]
  let unit: string
  let stroke: string
  let yMin: number | undefined
  let yMax: number | undefined
  let format: (v: number) => string
  switch (metric) {
    case 'inputV':
      data = points.map((p, i) => ({
        t: ts[i],
        v: p.input_voltage < 0 ? null : p.input_voltage,
        ps: psArr[i],
      }))
      unit = 'V'
      stroke = 'rgb(20 184 166)'
      format = (v) => v.toFixed(1)
      break
    case 'load':
      data = points.map((p, i) => ({
        t: ts[i],
        v: p.load_percent < 0 ? null : p.load_percent,
        ps: psArr[i],
      }))
      unit = '%'
      stroke = 'rgb(59 130 246)'
      yMin = 0
      yMax = 100
      format = (v) => `${Math.round(v)}`
      break
    case 'power':
      data = points.map((p, i) => ({
        t: ts[i],
        v: p.real_power < 0 ? null : p.real_power,
        ps: psArr[i],
      }))
      unit = 'W'
      stroke = 'rgb(234 88 12)'
      format = (v) => `${Math.round(v)}`
      break
  }
  return (
    <MiniChart data={data} unit={unit} stroke={stroke} yMin={yMin} yMax={yMax} format={format} />
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

  const bands = useMemo(() => {
    const out: { start: number; end: number; kind: 'battery' | 'low_battery' }[] = []
    let cur: { start: number; end: number; kind: 'battery' | 'low_battery' } | null = null
    for (const d of data) {
      if (d.ps === 'battery' || d.ps === 'low_battery') {
        const k = d.ps === 'low_battery' ? 'low_battery' : 'battery'
        if (!cur || cur.kind !== k) {
          if (cur) out.push(cur)
          cur = { start: d.t, end: d.t, kind: k }
        } else cur.end = d.t
      } else if (cur) {
        out.push(cur)
        cur = null
      }
    }
    if (cur) out.push(cur)
    return out
  }, [data])

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

  // y 范围:固定范围或自动适配(略带 padding)
  let yLo: number, yHi: number
  if (yMin != null && yMax != null) {
    yLo = yMin
    yHi = yMax
  } else if (allMissing) {
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

  // y 轴 3 个刻度
  const yTicks = [yLo, (yLo + yHi) / 2, yHi]

  // 折线路径(null 中断)
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
            <span className="ml-0.5">{unit}</span>
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
        {bands.map((b, i) => {
          const x1 = xOf(b.start)
          const x2 = Math.max(xOf(b.end), x1 + 2)
          return (
            <rect
              key={i}
              x={x1}
              y={padT}
              width={x2 - x1}
              height={innerH}
              fill={
                b.kind === 'low_battery'
                  ? 'rgba(244,63,94,0.18)'
                  : 'rgba(245,158,11,0.16)'
              }
            />
          )
        })}
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
