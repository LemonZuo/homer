import { useEffect, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react'

import { cn } from '../../../lib/utils'
import type { MiniPoint } from './types'

export function MiniChart({
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
  // 默认显示最近一个非空 bucket,作为基线读数;悬浮时切换为悬浮点。
  let defaultPoint: MiniPoint | null = null
  for (let i = data.length - 1; i >= 0; i--) {
    if (data[i].v != null) {
      defaultPoint = data[i]
      break
    }
  }
  const displayPoint = hoverPoint ?? defaultPoint
  const showing = displayPoint && displayPoint.v != null

  return (
    <div ref={wrapRef} className="relative">
      {/* 值槽始终占位,只切换内容与强弱,避免悬浮时整行尺寸跳动。 */}
      <div className="mb-1 flex h-4 items-center justify-end text-[11.5px]">
        {showing ? (
          <span className="tabular-nums">
            <span
              className={cn(
                'font-semibold',
                hoverPoint ? 'text-foreground' : 'text-foreground/70',
              )}
            >
              {format(displayPoint!.v as number)}
            </span>
            {unit && <span className="ml-0.5 text-muted-foreground">{unit}</span>}
            <span
              className={cn(
                'ml-2 tabular-nums',
                hoverPoint ? 'text-foreground' : 'text-muted-foreground/70',
              )}
            >
              {new Date(displayPoint!.t).toLocaleString('zh-CN', { hour12: false })}
            </span>
          </span>
        ) : (
          <span className="text-muted-foreground">{unit}</span>
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
