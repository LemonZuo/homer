import { useEffect, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react'

import type { LineSeries } from './types'

// MultiLineChart 多线版本的 mini chart。顶部 legend 横排,hover 时在 legend 上挂值;
// 主体单 svg 多 path,统一 y 轴(所有线共享 yLo/yHi)。结构刻意贴近 MiniChart,便于以后对齐。
export function MultiLineChart({
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
