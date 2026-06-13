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
  const [metric, setMetric] = useState<MetricKey>('voltage')
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

interface MiniSeries {
  label: string
  stroke: string
  data: MiniPoint[]
  dash?: string
  width?: number
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
  // 多张子图共享 hover idx,实现联动游标
  const [hoverIdx, setHoverIdx] = useState<number | null>(null)
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
  if (metric === 'voltage') {
    // 拆成上下两张子图,各自 y 轴自适应。两条线值常常贴近,挤在同一坐标系里看不清差异。
    const inputSeries: MiniSeries[] = [
      {
        label: '输入',
        stroke: 'rgb(20 184 166)',
        data: points.map((p, i) => ({
          t: ts[i],
          v: p.input_voltage < 0 ? null : p.input_voltage,
          ps: psArr[i],
        })),
      },
    ]
    const outputSeries: MiniSeries[] = [
      {
        label: '输出',
        stroke: 'rgb(168 85 247)',
        data: points.map((p, i) => ({
          t: ts[i],
          v: p.output_voltage < 0 ? null : p.output_voltage,
          ps: psArr[i],
        })),
      },
    ]
    return (
      <div className="space-y-2">
        <MiniChart
          series={inputSeries}
          unit="V"
          format={(v) => v.toFixed(1)}
          title="输入电压"
          height={116}
          showXAxis={false}
          clipExtremes
          hoverIdx={hoverIdx}
          onHover={setHoverIdx}
        />
        <MiniChart
          series={outputSeries}
          unit="V"
          format={(v) => v.toFixed(1)}
          title="输出电压"
          height={116}
          hoverIdx={hoverIdx}
          onHover={setHoverIdx}
        />
      </div>
    )
  }
  let series: MiniSeries[]
  let unit: string
  let yMin: number | undefined
  let yMax: number | undefined
  let format: (v: number) => string
  switch (metric) {
    case 'load':
      series = [
        {
          label: '负载',
          stroke: 'rgb(59 130 246)',
          data: points.map((p, i) => ({
            t: ts[i],
            v: p.load_percent < 0 ? null : p.load_percent,
            ps: psArr[i],
          })),
        },
      ]
      unit = '%'
      yMin = 0
      yMax = 100
      format = (v) => `${Math.round(v)}`
      break
    case 'power':
      series = [
        {
          label: '功率',
          stroke: 'rgb(234 88 12)',
          data: points.map((p, i) => ({
            t: ts[i],
            v: p.real_power < 0 ? null : p.real_power,
            ps: psArr[i],
          })),
        },
      ]
      unit = 'W'
      format = (v) => `${Math.round(v)}`
      break
  }
  return (
    <MiniChart
      series={series}
      unit={unit}
      yMin={yMin}
      yMax={yMax}
      format={format}
      height={260}
      hoverIdx={hoverIdx}
      onHover={setHoverIdx}
    />
  )
}

function MiniChart({
  unit,
  series,
  yMin,
  yMax,
  format,
  title,
  height,
  showXAxis = true,
  clipExtremes = false,
  hoverIdx,
  onHover,
}: {
  unit: string
  series: MiniSeries[]
  yMin?: number
  yMax?: number
  format: (v: number) => string
  title?: string
  height?: number
  showXAxis?: boolean
  // 开启后 y 范围按 P1~P99 分位裁剪,极端点(掉电瞬间等)不撑大 y 跨度,
  // 而是以红色 spike 表示"此处有跌穿但超出 y 范围",
  // 让正常态的细微波动能在 y 方向上充分展开。
  clipExtremes?: boolean
  hoverIdx: number | null
  onHover: (idx: number | null) => void
}) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(640)

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

  // 用第一条 series 的 ps 序列计算电源状态背景色带(所有 series 共享时间轴和 ps)
  const psSource = useMemo(() => series[0]?.data ?? [], [series])
  const bands = useMemo(() => {
    const out: { start: number; end: number; kind: 'battery' | 'low_battery' }[] = []
    let cur: { start: number; end: number; kind: 'battery' | 'low_battery' } | null = null
    for (const d of psSource) {
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
  }, [psSource])

  const allVals: number[] = []
  for (const s of series) {
    for (const d of s.data) {
      if (d.v != null) allVals.push(d.v)
    }
  }
  const allMissing = allVals.length === 0

  const H = height ?? 180
  const padL = 40
  const padR = 8
  const padT = 8
  const padB = showXAxis ? 18 : 6
  const innerW = Math.max(20, width - padL - padR)
  const innerH = H - padT - padB

  const tMin = psSource[0]?.t ?? 0
  const tMax = psSource[psSource.length - 1]?.t ?? 1
  const tSpan = Math.max(1, tMax - tMin)
  const xOf = (t: number) => padL + ((t - tMin) / tSpan) * innerW

  // y 范围:固定范围或自动适配(略带 padding)
  // clipExtremes=true 时按 P1~P99 分位裁剪极值,正常态波动充分展开;
  // 跌穿点(v < yLo)交给 spike 单独渲染,不撑大 y 跨度。
  let yLo: number, yHi: number
  if (yMin != null && yMax != null) {
    yLo = yMin
    yHi = yMax
  } else if (allMissing) {
    yLo = 0
    yHi = 1
  } else if (clipExtremes) {
    // 用 IQR(四分位距)裁剪:中位数 ±3*IQR,至少留 5V 窗口。
    // 跌穿持续若干采样点时,分位法(P1/P99)会被跌穿值拉偏;
    // 用 IQR 对极值鲁棒得多 —— 正常态 IQR 通常 1~2V,
    // 跌穿值偏离 100+ V,3*IQR 直接排除掉。
    const sorted = [...allVals].sort((a, b) => a - b)
    const q = (p: number) => {
      const idx = Math.min(sorted.length - 1, Math.max(0, Math.round((sorted.length - 1) * p)))
      return sorted[idx]
    }
    const p25 = q(0.25)
    const p75 = q(0.75)
    const iqr = Math.max(p75 - p25, 0.5)
    const halfRange = Math.max(iqr * 3, 5)
    yLo = p25 - halfRange
    yHi = p75 + halfRange
    if (yMin != null) yLo = yMin
    if (yMax != null) yHi = yMax
  } else {
    const mn = Math.min(...allVals)
    const mx = Math.max(...allVals)
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

  // 每条 series 一条折线;null 中断;clipExtremes 时跌穿点(v<yLo)也中断主曲线,
  // 改用 spike 单独表示。
  const spikes: number[] = [] // 跌穿点的 x 坐标
  const paths = series.map((s) => {
    let path = ''
    let penUp = true
    for (const d of s.data) {
      if (d.v == null) {
        penUp = true
        continue
      }
      if (clipExtremes && d.v < yLo) {
        spikes.push(xOf(d.t))
        penUp = true
        continue
      }
      const cmd = penUp ? 'M' : 'L'
      path += `${cmd}${xOf(d.t).toFixed(1)},${yOf(d.v).toFixed(1)} `
      penUp = false
    }
    return path.trim()
  })

  const xTickCount = Math.min(5, psSource.length)
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
      onHover(null)
      return
    }
    const ratio = (x - padL) / innerW
    const targetT = tMin + ratio * tSpan
    let bestIdx = 0
    let bestDiff = Infinity
    for (let i = 0; i < psSource.length; i++) {
      const diff = Math.abs(psSource[i].t - targetT)
      if (diff < bestDiff) {
        bestDiff = diff
        bestIdx = i
      }
    }
    onHover(bestIdx)
  }

  const hoverT = hoverIdx != null ? psSource[hoverIdx]?.t ?? null : null
  const hoverVals = hoverIdx != null
    ? series.map((s) => s.data[hoverIdx]?.v ?? null)
    : []
  const showMultiLegend = series.length > 1

  return (
    <div ref={wrapRef} className="relative">
      <div className="mb-1 flex h-4 flex-wrap items-center justify-between gap-x-3 gap-y-0 text-[11.5px] text-muted-foreground">
        <div className="flex items-center gap-3">
          {title && (
            <span className="inline-flex items-center gap-1">
              <span
                className="inline-block h-2 w-2 rounded-full"
                style={{ backgroundColor: series[0]?.stroke }}
              />
              <span>{title}</span>
            </span>
          )}
          {showMultiLegend &&
            series.map((s) => (
              <span key={s.label} className="inline-flex items-center gap-1">
                {s.dash ? (
                  <svg width="14" height="6" className="inline-block">
                    <line
                      x1="0"
                      y1="3"
                      x2="14"
                      y2="3"
                      stroke={s.stroke}
                      strokeWidth={2}
                      strokeDasharray="3 2"
                    />
                  </svg>
                ) : (
                  <span
                    className="inline-block h-2 w-2 rounded-full"
                    style={{ backgroundColor: s.stroke }}
                  />
                )}
                <span>{s.label}</span>
              </span>
            ))}
        </div>
        <div className="flex flex-wrap items-center gap-x-2 gap-y-0">
          {hoverT != null && hoverVals.some((v) => v != null) ? (
            <>
              {series.map((s, i) => {
                const v = hoverVals[i]
                if (v == null) return null
                return (
                  <span key={s.label} className="tabular-nums">
                    {showMultiLegend && (
                      <span style={{ color: s.stroke }} className="mr-1">
                        {s.label}
                      </span>
                    )}
                    <span className="text-foreground font-semibold">{format(v)}</span>
                    <span className="ml-0.5">{unit}</span>
                  </span>
                )
              })}
              <span className="text-muted-foreground">
                {new Date(hoverT).toLocaleString('zh-CN', { hour12: false })}
              </span>
            </>
          ) : (
            <span>{unit}</span>
          )}
        </div>
      </div>
      <svg
        width="100%"
        height={H}
        viewBox={`0 0 ${width} ${H}`}
        preserveAspectRatio="none"
        onMouseMove={onMove}
        onMouseLeave={() => onHover(null)}
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
        {showXAxis &&
          xTicks.map((t, i) => (
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
        {spikes.map((x, i) => (
          <line
            key={`spike-${i}`}
            x1={x}
            x2={x}
            y1={padT}
            y2={padT + innerH}
            stroke="rgb(244 63 94)"
            strokeWidth={1.5}
            opacity={0.85}
          />
        ))}
        {!allMissing &&
          // 倒序绘制:数组靠后的先画(在下),靠前的后画(在上)
          series
            .map((s, i) => ({ s, i }))
            .reverse()
            .map(({ s, i }) => (
              <path
                key={s.label}
                d={paths[i]}
                fill="none"
                stroke={s.stroke}
                strokeWidth={s.width ?? 1.75}
                strokeDasharray={s.dash}
                strokeLinecap={s.dash ? 'butt' : 'round'}
                strokeLinejoin="round"
              />
            ))}
        {hoverT != null && (
          <g>
            <line
              x1={xOf(hoverT)}
              x2={xOf(hoverT)}
              y1={padT}
              y2={padT + innerH}
              className="stroke-border"
              strokeWidth={1}
            />
            {series.map((s, i) => {
              const v = hoverVals[i]
              if (v == null) return null
              // 被 clip 的点不画圆点(会落到画布外),但顶部 tooltip 文字仍展示真实值
              if (clipExtremes && v < yLo) return null
              return (
                <circle
                  key={s.label}
                  cx={xOf(hoverT)}
                  cy={yOf(v)}
                  r={3.5}
                  fill={s.stroke}
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
