import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import {
  RefreshCw,
  Loader2,
  Settings2,
  Plug,
  AlertTriangle,
  ChevronDown,
  ChevronUp,
  Server,
  HardDrive,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { Switch } from './ui/switch'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from './ui/drawer'
import { cn } from '../lib/utils'

type PowerSource = 'mains' | 'battery' | 'low_battery' | 'unknown'

interface SnapshotUPS {
  name: string
  mfr: string
  model: string
  power_source: PowerSource
  battery_percent: number
  runtime_minutes: number
  input_voltage: number
  output_voltage: number
  load_percent: number
  real_power: number
  raw_status: string
  sampled_at: string
}

interface Snapshot {
  host_kind: string
  host_id: number
  host_name: string
  endpoint: string
  reachable: boolean
  error?: string
  upses: SnapshotUPS[]
}

interface Candidate {
  id: number
  name: string
  kind: string
  endpoint: string
  ups_monitor: boolean
}

interface SeriesPoint {
  bucket_start: string
  battery_percent: number
  runtime_minutes: number
  power_source: PowerSource
}

interface PowerMeta {
  label: string
  dot: string
  text: string
  pill: string
  pulse: boolean
}
const POWER_META: Record<PowerSource, PowerMeta> = {
  mains: {
    label: '市电供电',
    dot: 'bg-teal-500',
    text: 'text-teal-600 dark:text-teal-400',
    pill: 'border-teal-500/40 bg-teal-500/10 text-teal-700 dark:text-teal-300',
    pulse: false,
  },
  battery: {
    label: '电池供电',
    dot: 'bg-amber-500',
    text: 'text-amber-600 dark:text-amber-400',
    pill: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
    pulse: true,
  },
  low_battery: {
    label: '电量过低',
    dot: 'bg-rose-500',
    text: 'text-rose-600 dark:text-rose-400',
    pill: 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300',
    pulse: true,
  },
  unknown: {
    label: '状态未知',
    dot: 'bg-muted-foreground/60',
    text: 'text-muted-foreground',
    pill: 'border-border bg-muted text-muted-foreground',
    pulse: false,
  },
}

const HOST_KIND_LABEL: Record<string, string> = {
  ssh: 'SSH 机器',
  fnos: '飞牛 OS',
}

const RANGE_OPTIONS = [
  { value: '1h', label: '1 小时' },
  { value: '6h', label: '6 小时' },
  { value: '24h', label: '24 小时' },
  { value: '3d', label: '3 天' },
  { value: '7d', label: '7 天' },
]

function fmtRuntime(min: number): string {
  if (min < 0) return '— —'
  if (min < 60) return `${min} min`
  const h = Math.floor(min / 60)
  const m = min % 60
  return `${h}h ${m.toString().padStart(2, '0')}m`
}

function fmtTime(s: string): string {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`
}

function fmtNum(v: number, fractionDigits = 0): string {
  if (v == null || v < 0 || !isFinite(v)) return '—'
  return v.toFixed(fractionDigits)
}

function fmtDateTime(s: string): string {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// 垂直电池:≤20% rose / ≤50% amber / 否则 emerald;飞牛风方形圆角 + 一条清晰白光
function BatteryCell({ percent }: { percent: number }) {
  const hasData = percent >= 0
  const safe = hasData ? Math.max(0, Math.min(100, percent)) : 0
  const tone = !hasData
    ? { top: '#a1a1aa', bot: '#71717a' }
    : safe <= 20
      ? { top: '#f43f5e', bot: '#be123c' }
      : safe <= 50
        ? { top: '#f59e0b', bot: '#b45309' }
        : { top: '#00B26F', bot: '#009E61' }

  const W = 50
  const H = 60
  const r = 10
  const fillH = hasData ? (safe / 100) * H : 0
  const fillTop = H - fillH

  const uid = useId().replace(/:/g, '')
  const gradId = `bat-${uid}-g`
  const clipId = `bat-${uid}-c`

  return (
    <svg
      width={W}
      height={H}
      viewBox={`0 0 ${W} ${H}`}
      className="shrink-0 text-muted-foreground"
      aria-label={hasData ? `电量 ${safe}%` : '电量未知'}
    >
      <defs>
        <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={tone.top} />
          <stop offset="100%" stopColor={tone.bot} />
        </linearGradient>
        <clipPath id={clipId}>
          <rect x={0} y={0} width={W} height={H} rx={r} ry={r} />
        </clipPath>
      </defs>

      <rect width={W} height={H} rx={r} ry={r} fill="currentColor" opacity={0.1} />

      <g clipPath={`url(#${clipId})`}>
        {hasData && (
          <rect x={0} y={fillTop} width={W} height={fillH} fill={`url(#${gradId})`} />
        )}
      </g>

      {!hasData && (
        <text
          x={W / 2}
          y={H / 2 + 4}
          textAnchor="middle"
          fontSize={12}
          fill="currentColor"
          opacity={0.5}
        >
          —
        </text>
      )}
    </svg>
  )
}

function UPSCard({
  ups,
  hostKind,
  hostID,
}: {
  ups: SnapshotUPS
  hostKind: string
  hostID: number
}) {
  const [expanded, setExpanded] = useState(false)
  const [range, setRange] = useState('24h')
  const [series, setSeries] = useState<SeriesPoint[] | null>(null)
  const [seriesLoading, setSeriesLoading] = useState(false)

  const meta = POWER_META[ups.power_source] ?? POWER_META.unknown
  const hasData = ups.battery_percent >= 0
  const cs = getColorSet('teal')

  const loadSeries = useCallback(
    async (r: string) => {
      setSeriesLoading(true)
      try {
        const { data } = await api.get('/ups/series', {
          params: {
            host_kind: hostKind,
            host_id: hostID,
            ups_name: ups.name,
            range: r,
          },
        })
        setSeries(data?.data ?? [])
      } catch (e: any) {
        toast.error(e?.response?.data?.error || e?.message || '加载历史失败')
      } finally {
        setSeriesLoading(false)
      }
    },
    [hostKind, hostID, ups.name],
  )

  useEffect(() => {
    if (!expanded) return
    loadSeries(range)
  }, [expanded, range, loadSeries])

  return (
    <Card className={cn('group transition-[transform,box-shadow,border-color] duration-500 ease-out hover:-translate-y-0.5', cs.border, cs.halo)}>
      <div className="p-5">
        {/* 顶部 */}
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="rounded-md border border-border bg-muted px-1.5 py-0.5 text-[11px] font-medium tabular-nums text-muted-foreground">
                {ups.name}
              </span>
              {ups.mfr && (
                <span className="truncate text-[12px] text-muted-foreground">{ups.mfr}</span>
              )}
            </div>
            <div
              className="mt-1 truncate text-[15px] font-semibold tracking-tight"
              title={ups.model}
            >
              {ups.model || '(无型号)'}
            </div>
          </div>
          <span
            className={cn(
              'shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-medium',
              meta.pill,
            )}
          >
            <span className="inline-flex items-center gap-1.5">
              <span
                className={cn(
                  'h-1.5 w-1.5 rounded-full',
                  meta.dot,
                  meta.pulse && 'animate-pulse',
                )}
              />
              {meta.label}
            </span>
          </span>
        </div>

        {/* 主体 */}
        <div className="mt-5 flex items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <BatteryCell percent={ups.battery_percent} />
            <div>
              {hasData ? (
                <div className={cn('flex items-baseline gap-1 leading-none', meta.text)}>
                  <span className="text-[44px] font-semibold tabular-nums tracking-tight sm:text-[52px]">
                    {ups.battery_percent}
                  </span>
                  <span className="text-xl font-semibold">%</span>
                </div>
              ) : (
                <span className="text-[36px] font-semibold leading-none tabular-nums text-muted-foreground/60">
                  — —
                </span>
              )}
              <div className="mt-1.5 text-[11.5px] text-muted-foreground">剩余电量</div>
            </div>
          </div>
          <div className="text-right">
            <div className={cn('text-2xl font-semibold leading-none tabular-nums sm:text-3xl', meta.text)}>
              {fmtRuntime(ups.runtime_minutes)}
            </div>
            <div className="mt-1.5 text-[11.5px] text-muted-foreground">预估可供电</div>
          </div>
        </div>

        {/* 电气指标:输入电压 / 负载 / 实时功率 */}
        {(ups.input_voltage >= 0 || ups.load_percent >= 0 || ups.real_power >= 0) && (
          <div className="mt-5 grid grid-cols-3 gap-2 rounded-md border border-border/60 bg-muted/30 px-3 py-2.5">
            <div className="min-w-0">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">输入电压</div>
              <div className="mt-0.5 flex items-baseline gap-0.5 leading-none">
                <span className="text-[15px] font-semibold tabular-nums text-foreground">
                  {fmtNum(ups.input_voltage, 1)}
                </span>
                <span className="text-[10.5px] text-muted-foreground">V</span>
              </div>
            </div>
            <div className="min-w-0 border-l border-border/60 pl-3">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">负载</div>
              <div className="mt-0.5 flex items-baseline gap-0.5 leading-none">
                <span className="text-[15px] font-semibold tabular-nums text-foreground">
                  {fmtNum(ups.load_percent)}
                </span>
                <span className="text-[10.5px] text-muted-foreground">%</span>
              </div>
            </div>
            <div className="min-w-0 border-l border-border/60 pl-3">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">实时功率</div>
              <div className="mt-0.5 flex items-baseline gap-0.5 leading-none">
                <span className="text-[15px] font-semibold tabular-nums text-foreground">
                  {fmtNum(ups.real_power)}
                </span>
                <span className="text-[10.5px] text-muted-foreground">W</span>
              </div>
            </div>
          </div>
        )}

        {/* 底部信息 */}
        <div className="mt-3 flex items-center justify-between gap-2 border-t border-border/60 pt-3 text-[11.5px]">
          <div className="flex min-w-0 items-center gap-2 text-muted-foreground">
            {ups.raw_status ? (
              <span className="truncate" title={ups.raw_status}>
                状态码 <code className="rounded bg-muted px-1 py-0.5 text-[10.5px] text-foreground/80">{ups.raw_status}</code>
              </span>
            ) : (
              <span className="text-muted-foreground/70">尚未收到状态码</span>
            )}
          </div>
          <div className="shrink-0 tabular-nums text-muted-foreground">
            {fmtTime(ups.sampled_at)}
          </div>
        </div>

        {/* 展开/收起 */}
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="mt-3 flex w-full items-center justify-center gap-1 rounded-md border border-dashed border-border py-1.5 text-[11.5px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        >
          {expanded ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
          {expanded ? '收起历史' : '查看历史'}
        </button>

        {expanded && (
          <div className="mt-3 rounded-md border border-border/60 bg-muted/30 p-3">
            <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
              <span className="text-[11.5px] text-muted-foreground">电量曲线</span>
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
            <SeriesChart points={series} loading={seriesLoading} />
          </div>
        )}
      </div>
    </Card>
  )
}

interface ChartPoint {
  t: number
  battery: number | null
  ps: PowerSource
}

function SeriesChart({ points, loading }: { points: SeriesPoint[] | null; loading: boolean }) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(640)
  const [hover, setHover] = useState<{ x: number; idx: number } | null>(null)

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

  const data = useMemo<ChartPoint[]>(
    () =>
      (points ?? []).map((p) => ({
        t: new Date(p.bucket_start).getTime(),
        battery: p.battery_percent < 0 ? null : p.battery_percent,
        ps: p.power_source,
      })),
    [points],
  )

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

  if (loading) {
    return (
      <div className="flex h-44 items-center justify-center text-[12px] text-muted-foreground">
        <Loader2 className="mr-1.5 h-3 w-3 animate-spin" />
        加载中
      </div>
    )
  }
  if (!points || points.length === 0 || data.length === 0) {
    return (
      <div className="flex h-44 items-center justify-center text-[12px] text-muted-foreground">
        暂无历史数据
      </div>
    )
  }

  const H = 176
  const padL = 32
  const padR = 8
  const padT = 8
  const padB = 18
  const innerW = Math.max(20, width - padL - padR)
  const innerH = H - padT - padB

  const tMin = data[0].t
  const tMax = data[data.length - 1].t
  const tSpan = Math.max(1, tMax - tMin)
  const xOf = (t: number) => padL + ((t - tMin) / tSpan) * innerW
  const yOf = (v: number) => padT + (1 - v / 100) * innerH

  // 折线路径(允许 null 中断)
  let path = ''
  let penUp = true
  for (const d of data) {
    if (d.battery == null) {
      penUp = true
      continue
    }
    const cmd = penUp ? 'M' : 'L'
    path += `${cmd}${xOf(d.t).toFixed(1)},${yOf(d.battery).toFixed(1)} `
    penUp = false
  }

  const yTicks = [0, 25, 50, 75, 100]
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

  // 悬停
  const onMove = (ev: React.MouseEvent<SVGSVGElement>) => {
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
    setHover({ x, idx: bestIdx })
  }

  const hoverPoint = hover ? data[hover.idx] : null

  return (
    <div ref={wrapRef} className="relative h-44 w-full">
      <svg
        width="100%"
        height={H}
        viewBox={`0 0 ${width} ${H}`}
        preserveAspectRatio="none"
        onMouseMove={onMove}
        onMouseLeave={() => setHover(null)}
      >
        {/* Y 轴网格 + 刻度 */}
        {yTicks.map((v) => (
          <g key={v}>
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
              {v}%
            </text>
          </g>
        ))}

        {/* 事件色带 */}
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

        {/* X 轴刻度 */}
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

        {/* 折线 */}
        <path
          d={path.trim()}
          fill="none"
          stroke="rgb(20 184 166)"
          strokeWidth={1.75}
          strokeLinecap="round"
          strokeLinejoin="round"
        />

        {/* 悬停指示 */}
        {hoverPoint && hoverPoint.battery != null && (
          <g>
            <line
              x1={xOf(hoverPoint.t)}
              x2={xOf(hoverPoint.t)}
              y1={padT}
              y2={padT + innerH}
              className="stroke-border"
              strokeWidth={1}
            />
            <circle
              cx={xOf(hoverPoint.t)}
              cy={yOf(hoverPoint.battery)}
              r={3.5}
              fill="rgb(20 184 166)"
            />
          </g>
        )}
      </svg>

      {hoverPoint && hoverPoint.battery != null && (
        <div
          className="pointer-events-none absolute top-1 rounded-md border border-border bg-popover px-2 py-1 text-[11px] text-popover-foreground shadow-md"
          style={{
            left: Math.min(width - 160, Math.max(0, xOf(hoverPoint.t) + 6)),
          }}
        >
          <div className="tabular-nums text-muted-foreground">
            {new Date(hoverPoint.t).toLocaleString('zh-CN', { hour12: false })}
          </div>
          <div className="mt-0.5">
            电量 <span className="font-semibold text-foreground">{hoverPoint.battery}%</span>
            {hoverPoint.ps !== 'mains' && (
              <span className={cn('ml-2', POWER_META[hoverPoint.ps].text)}>
                {POWER_META[hoverPoint.ps].label}
              </span>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function HostBlock({ host }: { host: Snapshot }) {
  const kindLabel = HOST_KIND_LABEL[host.host_kind] ?? host.host_kind
  const noUPS = host.upses.length === 0
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <Server className="h-4 w-4 text-muted-foreground" />
        <span className="text-[14px] font-semibold tracking-tight">{host.host_name}</span>
        <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
          {kindLabel}
        </span>
        <span className="text-[12px] text-muted-foreground">{host.endpoint}</span>
        {!host.reachable && noUPS && (
          <span className="ml-auto flex items-center gap-1 text-[12px] text-rose-500">
            <AlertTriangle className="h-3.5 w-3.5" />
            {host.error ? '采样失败' : '尚无数据'}
          </span>
        )}
      </div>
      {noUPS ? (
        <Card className="px-4 py-6 text-center text-[12px] text-muted-foreground">
          {host.error
            ? `采样失败:${host.error}`
            : '尚未采集到 UPS 数据(机器未装 NUT / 未绑 UPS / 等待首次采样)'}
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
          {host.upses.map((u) => (
            <UPSCard key={u.name} ups={u} hostKind={host.host_kind} hostID={host.host_id} />
          ))}
        </div>
      )}
    </div>
  )
}

// 监听 media query。与 Layout.tsx 同款实现:Drawer 内部用来选 picker 列数档位。
function useMediaQuery(query: string) {
  const [matches, setMatches] = useState(() =>
    typeof window === 'undefined' ? false : window.matchMedia(query).matches,
  )
  useEffect(() => {
    const media = window.matchMedia(query)
    const onChange = () => setMatches(media.matches)
    onChange()
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [query])
  return matches
}

// 订阅列表网格按 picker 同算法:grid-cols-6 + col-span,余数项把最后一行均分占满。
// 1 列档不会触发 tail,SPAN_CLASS[6] 退化为整行。
const SPAN_CLASS: Record<number, string> = {
  2: 'col-span-2',
  3: 'col-span-3',
  6: 'col-span-6',
}

function SubscriptionItem({
  candidate,
  pending,
  onToggle,
}: {
  candidate: Candidate
  pending: boolean
  onToggle: (enable: boolean) => void
}) {
  const Icon = candidate.kind === 'fnos' ? HardDrive : Server
  return (
    <Card className="px-3 py-2.5">
      <div className="flex items-center gap-3">
        <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
        <div className="min-w-0 flex-1">
          <div className="truncate font-mono text-[13px] font-medium">{candidate.name}</div>
          <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
            {candidate.endpoint}
          </div>
        </div>
        <Switch
          checked={candidate.ups_monitor}
          onChange={onToggle}
          disabled={pending}
        />
      </div>
    </Card>
  )
}

function SubscriptionsDrawer({
  open,
  onOpenChange,
  onChanged,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  onChanged: () => void
}) {
  const [candidates, setCandidates] = useState<Candidate[]>([])
  const [loading, setLoading] = useState(false)
  const [pending, setPending] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/ups/candidates')
      setCandidates(data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open) load()
  }, [open, load])

  const toggle = async (c: Candidate, enable: boolean) => {
    setPending(c.id)
    try {
      await api.post(`/ups/candidates/${c.id}/toggle`, { enable })
      setCandidates((prev) => prev.map((x) => (x.id === c.id ? { ...x, ups_monitor: enable } : x)))
      onChanged()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '切换失败')
    } finally {
      setPending(null)
    }
  }

  const sshList = candidates.filter((c) => c.kind === 'ssh')
  const fnosList = candidates.filter((c) => c.kind === 'fnos')

  // 与 Layout 的 picker 一致:430px 起 2 列,1024px 起 3 列
  const wide = useMediaQuery('(min-width: 430px)')
  const desktop = useMediaQuery('(min-width: 1024px)')
  const cols = desktop ? 3 : wide ? 2 : 1

  const renderGroup = (list: Candidate[]) => {
    const LCM = 6
    const itemSpan = LCM / cols
    const tail = list.length % cols
    const tailSpan = tail > 0 ? LCM / tail : itemSpan
    return (
      <div className="grid grid-cols-6 gap-2">
        {list.map((c, idx) => {
          const isInTail = tail > 0 && idx >= list.length - tail
          const span = isInTail ? tailSpan : itemSpan
          return (
            <div key={c.id} className={SPAN_CLASS[span] ?? 'col-span-6'}>
              <SubscriptionItem
                candidate={c}
                pending={pending === c.id}
                onToggle={(v) => toggle(c, v)}
              />
            </div>
          )
        })}
      </div>
    )
  }

  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>UPS 监控机器订阅</DrawerTitle>
          <DrawerDescription>
            勾选哪些 SSH / 飞牛 OS 目标参与 UPS 状态采样。未勾选的机器不会被 SSH 拨号打扰。
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-5 overflow-auto px-4 pb-6">
          {loading && (
            <div className="flex items-center justify-center py-8 text-[12px] text-muted-foreground">
              <Loader2 className="mr-1.5 h-3 w-3 animate-spin" />
              加载中
            </div>
          )}
          {!loading && candidates.length === 0 && (
            <div className="py-8 text-center text-[12.5px] text-muted-foreground">
              尚未在 ACME 部署目标里添加任何 SSH / 飞牛 OS 机器。
            </div>
          )}

          {sshList.length > 0 && (
            <div className="space-y-2">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <div className="text-[13px] font-medium">SSH 机器</div>
                  <div className="text-[11.5px] text-muted-foreground">{sshList.length} 台机器</div>
                </div>
              </div>
              {renderGroup(sshList)}
            </div>
          )}

          {fnosList.length > 0 && (
            <div className={cn('space-y-2', sshList.length > 0 && 'border-t border-border pt-5')}>
              <div className="flex items-center justify-between gap-3">
                <div>
                  <div className="text-[13px] font-medium">飞牛 OS</div>
                  <div className="text-[11.5px] text-muted-foreground">{fnosList.length} 个实例</div>
                </div>
              </div>
              {renderGroup(fnosList)}
            </div>
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

export default function Ups() {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [subOpen, setSubOpen] = useState(false)

  const normalize = (arr: any[]): Snapshot[] =>
    (arr ?? []).map((s) => ({ ...s, upses: s?.upses ?? [] }))

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/ups/snapshot')
      setSnapshots(normalize(data?.data))
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  const triggerSample = useCallback(async () => {
    setRefreshing(true)
    try {
      const { data } = await api.post('/ups/refresh')
      setSnapshots(normalize(data?.data))
      toast.success('已触发一次采样')
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '采样失败')
    } finally {
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  // SSE 推送替代 30 秒轮询。订阅时后端立即发首帧,之后每轮采样完推一帧;
  // EventSource 内置断线重连(默认 3s),所以这里不用自己写 retry。
  useEffect(() => {
    const es = new EventSource('/api/ups/stream')
    es.addEventListener('snapshot', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data)
        setSnapshots(normalize(data))
        setLoading(false)
      } catch {
        // 忽略损坏的单帧,等下一帧
      }
    })
    return () => es.close()
  }, [])

  const cs = getColorSet('teal')
  const empty = !loading && snapshots.length === 0

  const stats = useMemo(() => {
    let hosts = snapshots.length
    let upses = 0
    let alerts = 0
    for (const s of snapshots) {
      for (const u of s.upses) {
        upses++
        if (u.power_source === 'battery' || u.power_source === 'low_battery') alerts++
      }
    }
    return { hosts, upses, alerts }
  }, [snapshots])

  // 最近一次采样时间(取所有 UPS 里最新的)
  const lastSampled = useMemo(() => {
    let latest = ''
    for (const s of snapshots) {
      for (const u of s.upses) {
        if (!latest || u.sampled_at > latest) latest = u.sampled_at
      }
    }
    return latest
  }, [snapshots])

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="hidden sm:block">
          <div className="flex items-center gap-3">
            <span className={cn('h-2 w-2 rounded-full', cs.dot)} />
            <h1 className="text-[28px] font-bold leading-none tracking-tight">UPS 状态</h1>
          </div>
          <p className="mt-2 text-[12.5px] text-muted-foreground">
            {stats.hosts} 台机器 / {stats.upses} 台 UPS
            {stats.alerts > 0 && (
              <span className="ml-2 inline-flex items-center gap-1 text-rose-500">
                <AlertTriangle className="h-3 w-3" />
                {stats.alerts} 台正在电池供电
              </span>
            )}
            {lastSampled && (
              <span className="ml-2 text-muted-foreground/70">· 最近采样 {fmtDateTime(lastSampled)}</span>
            )}
          </p>
        </div>
        <div className="flex shrink-0 gap-2">
          <Button
            variant="outline"
            size="sm"
            className="flex-1 sm:flex-none"
            onClick={triggerSample}
            disabled={refreshing}
          >
            {refreshing ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
            )}
            立即采样
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="flex-1 sm:flex-none"
            onClick={() => setSubOpen(true)}
          >
            <Settings2 className="mr-1.5 h-3.5 w-3.5" />
            机器订阅
          </Button>
        </div>
      </div>

      {loading ? (
        <Card className="px-4 py-16 text-center text-[12.5px] text-muted-foreground">
          <Loader2 className="mx-auto mb-2 h-4 w-4 animate-spin" />
          加载中
        </Card>
      ) : empty ? (
        <Card className="space-y-3 px-6 py-10 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-muted">
            <Plug className="h-5 w-5 text-muted-foreground" />
          </div>
          <div className="text-[14px] font-medium">尚未订阅任何机器</div>
          <p className="mx-auto max-w-md text-[12.5px] text-muted-foreground">
            点右上"机器订阅"勾选要参与 UPS 采样的 SSH / 飞牛 OS 目标。机器需先在 ACME 部署目标中存在,
            并在远端安装 NUT(<code className="rounded bg-muted px-1">nut-client</code>) +
            <code className="ml-1 rounded bg-muted px-1">upsc</code>。
          </p>
          <Button variant="outline" size="sm" onClick={() => setSubOpen(true)}>
            <Settings2 className="mr-1.5 h-3.5 w-3.5" />
            打开机器订阅
          </Button>
        </Card>
      ) : (
        <div className="space-y-8">
          {snapshots.map((s) => (
            <HostBlock key={`${s.host_kind}-${s.host_id}`} host={s} />
          ))}
        </div>
      )}

      <SubscriptionsDrawer open={subOpen} onOpenChange={setSubOpen} onChanged={load} />
    </div>
  )
}
