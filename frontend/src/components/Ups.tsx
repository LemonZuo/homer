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
  X,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { cn } from '../lib/utils'
import { UpsHostsDrawer } from './ups/UpsHostsDrawer'
import { UpsHostEditDialog } from './ups/UpsHostEditDialog'
import { UpsCredentialsDrawer } from './ups/UpsCredentialsDrawer'
import { UpsCredentialEditDialog } from './ups/UpsCredentialEditDialog'
import { DEMO_BATTERY_VARIANTS, POWER_META } from './ups/constants'
import { extractErr, fmtBatteryType, fmtDateTime, fmtNum, fmtRuntime, fmtStaleAge, fmtTime, isStaleSample, useNowTick } from './ups/format'
import type { PowerSource, Snapshot, SnapshotUPS, UpsCredential, UpsHost } from './ups/types'
import { SeriesChart } from './ups/history/SeriesChart'

// 垂直电池:≤20% rose / ≤50% amber / 否则 emerald;飞牛风方形圆角 + 一条清晰白光。
// 动效按 power_source 区分:mains 走上升扫描光(在线/充电感),battery/low_battery
// 走整体呼吸 opacity(正在被消耗),unknown / 离线无动效。
function BatteryCell({
  percent,
  powerSource = 'unknown',
  charging = false,
}: {
  percent: number
  powerSource?: PowerSource
  charging?: boolean
}) {
  const hasData = percent >= 0
  const safe = hasData ? Math.max(0, Math.min(100, percent)) : 0
  const tone = !hasData
    ? { top: '#a1a1aa', bot: '#71717a' }
    : safe <= 20
      ? { top: '#f43f5e', bot: '#be123c' }
      : safe <= 50
        ? { top: '#f59e0b', bot: '#b45309' }
        : { top: '#34d399', bot: '#10b981' }

  const W = 50
  const H = 60
  const r = 10
  const fillH = hasData ? (safe / 100) * H : 0
  const fillTop = H - fillH

  const uid = useId().replace(/:/g, '')
  const gradId = `bat-${uid}-g`
  const clipId = `bat-${uid}-c`

  // mains 走极轻液面呼吸,battery / low_battery 走更明显的脉动,lb 节奏更急。
  const showMainsBreath = hasData && powerSource === 'mains'
  const showBlink = hasData && (powerSource === 'battery' || powerSource === 'low_battery')
  const blinkDur = powerSource === 'low_battery' ? '1.1s' : '2s'

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
          <rect x={0} y={fillTop} width={W} height={fillH} fill={`url(#${gradId})`}>
            {showBlink && (
              <animate
                attributeName="opacity"
                values="1;0.55;1"
                dur={blinkDur}
                repeatCount="indefinite"
              />
            )}
            {showMainsBreath && (
              <animate
                attributeName="opacity"
                values="1;0.72;1"
                dur="5s"
                calcMode="spline"
                keySplines="0.4 0 0.6 1;0.4 0 0.6 1"
                repeatCount="indefinite"
              />
            )}
          </rect>
        )}
      </g>

      {/* 充电闪电:浮在液面之上,淡黄填 + 深色描边保证任意背景对比;opacity 缓脉动。*/}
      {hasData && charging && (
        <g transform="translate(8.2 13.2) scale(1.4)" opacity={0.9}>
          <path
            d="M13 2 L3 14 L12 14 L11 22 L21 10 L12 10 L13 2 Z"
            fill="#fef08a"
            stroke="#ca8a04"
            strokeWidth={0.6}
            strokeLinejoin="round"
            vectorEffect="non-scaling-stroke"
          />
          <animate
            attributeName="opacity"
            values="0.5;1;0.5"
            dur="2.4s"
            repeatCount="indefinite"
          />
        </g>
      )}

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
  const meta = POWER_META[ups.power_source] ?? POWER_META.unknown
  const hasData = ups.battery_percent >= 0
  const cs = getColorSet('teal')
  const now = useNowTick()
  const sampledAtMs = ups.sampled_at ? new Date(ups.sampled_at).getTime() : 0
  const isStale = isStaleSample(ups.sampled_at, now)
  const staleAge = isStale && sampledAtMs > 0 ? fmtStaleAge(now - sampledAtMs) : ''

  return (
    <Card className={cn(
      'group transition-[transform,box-shadow,border-color,opacity,filter] duration-500 ease-out hover:-translate-y-0.5',
      cs.border,
      cs.halo,
      isStale && 'opacity-60 saturate-50',
    )}>
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
          {isStale ? (
            <span
              className="shrink-0 rounded-full border border-border bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground"
              title={ups.sampled_at ? `最近采样 ${ups.sampled_at}` : '未拿到任何采样'}
            >
              <span className="inline-flex items-center gap-1.5">
                <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/60" />
                已离线{staleAge && ` · ${staleAge}`}
              </span>
            </span>
          ) : (
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
          )}
        </div>

        {/* 主体 */}
        <div className="mt-5 flex items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <BatteryCell
              percent={ups.battery_percent}
              powerSource={ups.power_source}
              charging={/\bCHRG\b/.test((ups.raw_status ?? '').toUpperCase())}
            />
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

        {/* 电池信息:类型 / 电压 / 额定电压 */}
        {(ups.battery_type || ups.battery_voltage >= 0 || ups.battery_nominal_voltage > 0) && (
          <div className="mt-5 grid grid-cols-3 gap-2 rounded-md border border-border/60 bg-muted/30 px-3 py-2.5">
            <div className="min-w-0">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">电池类型</div>
              <div className="mt-0.5 leading-none" title={ups.battery_type || undefined}>
                <span className="text-[14px] font-semibold text-foreground">
                  {fmtBatteryType(ups.battery_type)}
                </span>
              </div>
            </div>
            <div className="min-w-0 border-l border-border/60 pl-3">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">电池电压</div>
              <div className="mt-0.5 flex items-baseline gap-0.5 leading-none">
                <span className="text-[15px] font-semibold tabular-nums text-foreground">
                  {fmtNum(ups.battery_voltage, 1)}
                </span>
                <span className="text-[10.5px] text-muted-foreground">V</span>
              </div>
            </div>
            <div className="min-w-0 border-l border-border/60 pl-3">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">额定电压</div>
              <div className="mt-0.5 flex items-baseline gap-0.5 leading-none">
                <span className="text-[15px] font-semibold tabular-nums text-foreground">
                  {fmtNum(ups.battery_nominal_voltage, 0)}
                </span>
                <span className="text-[10.5px] text-muted-foreground">V</span>
              </div>
            </div>
          </div>
        )}

        {/* 电气指标:输入电压 / 负载 / 实时功率 */}
        {(ups.input_voltage >= 0 || ups.load_percent >= 0 || ups.real_power >= 0) && (
          <div className="mt-3 grid grid-cols-3 gap-2 rounded-md border border-border/60 bg-muted/30 px-3 py-2.5">
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

        <SeriesChart hostKind={hostKind} hostID={hostID} upsName={ups.name} expanded={expanded} />
      </div>
    </Card>
  )
}

function HostHeader({ host }: { host: Snapshot }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Server className="h-4 w-4 text-muted-foreground" />
      <span className="text-[14px] font-semibold tracking-tight">{host.host_name}</span>
      <span className="text-[12px] text-muted-foreground">{host.endpoint}</span>
      {!host.reachable && host.upses.length === 0 && (
        <span className="ml-auto flex items-center gap-1 text-[12px] text-rose-500">
          <AlertTriangle className="h-3.5 w-3.5" />
          {host.error ? '采样失败' : '尚无数据'}
        </span>
      )}
    </div>
  )
}

function HostEmptyCard({ host }: { host: Snapshot }) {
  return (
    <Card className="px-4 py-6 text-center text-[12px] text-muted-foreground">
      {host.error
        ? `采样失败:${host.error}`
        : '尚未采集到 UPS 数据(机器未装 NUT / 未绑 UPS / 等待首次采样)'}
    </Card>
  )
}

function DemoSection({ onClose }: { onClose: () => void }) {
  // 演示卡的 sampled_at 跟着 useNowTick 滴答推进:
  //   demo-offline → now - 15min,始终展示离线样式
  //   其余卡        → now,始终保持新鲜
  const now = useNowTick()
  const demoUpses = useMemo<SnapshotUPS[]>(
    () =>
      DEMO_BATTERY_VARIANTS.map((u) => ({
        ...u,
        sampled_at: new Date(u.name === 'demo-offline' ? now - 15 * 60 * 1000 : now).toISOString(),
      })),
    [now],
  )
  const demoSnapshots: Snapshot[] = [
    {
      host_kind: 'demo',
      host_id: -1,
      host_name: '演示机器',
      endpoint: 'demo:0',
      reachable: true,
      upses: demoUpses,
    },
  ]
  return (
    <div className="mt-10 rounded-2xl border border-dashed border-muted-foreground/30 bg-muted/20 p-4 sm:p-5">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <div className="text-[14px] font-semibold">样式演示</div>
          <div className="mt-0.5 text-[11.5px] text-muted-foreground">
            非真实数据,仅展示聚合总览卡、不同电量 / 电源 / 充电状态、以及超过 10 分钟未上报的离线样式。
          </div>
        </div>
        <Button variant="ghost" size="sm" onClick={onClose}>
          <X className="mr-1 h-3.5 w-3.5" />
          退出演示
        </Button>
      </div>
      <SummaryCard snapshots={demoSnapshots} />
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        {demoUpses.map((u) => (
          <UPSCard key={u.name} ups={u} hostKind="demo" hostID={-1} />
        ))}
      </div>
    </div>
  )
}

function SummaryCard({ snapshots }: { snapshots: Snapshot[] }) {
  const now = useNowTick()
  const agg = useMemo(() => {
    let mainsCount = 0
    let batteryCount = 0
    let lowCount = 0
    let offlineCount = 0
    let totalPower = 0
    let maxLoad = -1
    let minRuntime = -1
    for (const s of snapshots) {
      for (const u of s.upses) {
        if (isStaleSample(u.sampled_at, now)) {
          offlineCount++
          continue
        }
        if (u.power_source === 'mains') mainsCount++
        else if (u.power_source === 'battery') batteryCount++
        else if (u.power_source === 'low_battery') lowCount++
        if (u.real_power > 0) totalPower += u.real_power
        if (u.load_percent >= 0 && u.load_percent > maxLoad) maxLoad = u.load_percent
        if (u.runtime_minutes > 0 && (minRuntime < 0 || u.runtime_minutes < minRuntime)) {
          minRuntime = u.runtime_minutes
        }
      }
    }
    return { mainsCount, batteryCount, lowCount, offlineCount, totalPower, maxLoad, minRuntime }
  }, [snapshots, now])

  const alerts = agg.batteryCount + agg.lowCount

  return (
    <Card className="mb-6 grid grid-cols-2 gap-x-4 gap-y-4 px-4 py-4 sm:grid-cols-4 sm:gap-x-6 sm:px-6 sm:py-5">
      <div>
        <div className="text-[11px] uppercase tracking-wider text-muted-foreground">状态</div>
        <div className="mt-1.5 flex items-baseline gap-1.5">
          <span
            className={cn(
              'text-[22px] font-semibold leading-none tabular-nums',
              alerts > 0 ? 'text-foreground' : 'text-teal-600 dark:text-teal-400',
            )}
          >
            {agg.mainsCount}
          </span>
          <span className="text-[12px] text-muted-foreground">正常</span>
          {alerts > 0 && (
            <>
              <span className="text-[12px] text-muted-foreground/60">/</span>
              <span
                className={cn(
                  'text-[18px] font-semibold leading-none tabular-nums',
                  agg.lowCount > 0 ? 'text-rose-500' : 'text-amber-500',
                )}
              >
                {alerts}
              </span>
              <span className="text-[12px] text-muted-foreground">告警</span>
            </>
          )}
          {agg.offlineCount > 0 && (
            <>
              <span className="text-[12px] text-muted-foreground/60">/</span>
              <span className="text-[18px] font-semibold leading-none tabular-nums text-muted-foreground">
                {agg.offlineCount}
              </span>
              <span className="text-[12px] text-muted-foreground">离线</span>
            </>
          )}
        </div>
      </div>
      <div>
        <div className="text-[11px] uppercase tracking-wider text-muted-foreground">总实时功率</div>
        <div className="mt-1.5 flex items-baseline gap-1">
          <span className="text-[22px] font-semibold leading-none tabular-nums">
            {agg.totalPower > 0 ? agg.totalPower.toFixed(0) : '—'}
          </span>
          {agg.totalPower > 0 && <span className="text-[12px] text-muted-foreground">W</span>}
        </div>
      </div>
      <div>
        <div className="text-[11px] uppercase tracking-wider text-muted-foreground">最大负载</div>
        <div className="mt-1.5 flex items-baseline gap-1">
          <span className="text-[22px] font-semibold leading-none tabular-nums">
            {agg.maxLoad >= 0 ? agg.maxLoad.toFixed(0) : '—'}
          </span>
          {agg.maxLoad >= 0 && <span className="text-[12px] text-muted-foreground">%</span>}
        </div>
      </div>
      <div>
        <div className="text-[11px] uppercase tracking-wider text-muted-foreground">最短续航</div>
        <div className="mt-1.5 text-[22px] font-semibold leading-none tabular-nums">
          {agg.minRuntime > 0 ? fmtRuntime(agg.minRuntime) : '—'}
        </div>
      </div>
    </Card>
  )
}

export default function Ups() {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [hostsOpen, setHostsOpen] = useState(false)
  const [hostEditOpen, setHostEditOpen] = useState(false)
  const [editingHost, setEditingHost] = useState<UpsHost | null>(null)
  const [credsOpen, setCredsOpen] = useState(false)
  const [credEditOpen, setCredEditOpen] = useState(false)
  const [editingCred, setEditingCred] = useState<UpsCredential | null>(null)
  const [hosts, setHosts] = useState<UpsHost[]>([])
  const [credentials, setCredentials] = useState<UpsCredential[]>([])
  const [demoMode, setDemoMode] = useState(false)
  const demoTapRef = useRef<{ count: number; timer: ReturnType<typeof setTimeout> | null }>({
    count: 0,
    timer: null,
  })
  // 标题左侧状态点 5 秒内连点 5 次进入演示模式;再次进入只需关闭按钮退出。
  const bumpDemoTap = useCallback(() => {
    if (demoMode) return
    const s = demoTapRef.current
    s.count += 1
    if (s.timer) clearTimeout(s.timer)
    s.timer = setTimeout(() => {
      s.count = 0
      s.timer = null
    }, 5000)
    if (s.count >= 5) {
      s.count = 0
      if (s.timer) clearTimeout(s.timer)
      s.timer = null
      setDemoMode(true)
    }
  }, [demoMode])

  const normalize = (arr: unknown): Snapshot[] => {
    if (!Array.isArray(arr)) return []
    return arr.map((s) => {
      const obj = (s ?? {}) as Snapshot & { upses?: SnapshotUPS[] }
      return { ...obj, upses: obj.upses ?? [] }
    })
  }

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/ups/snapshot')
      setSnapshots(normalize(data?.data))
    } catch (e) {
      toast.error(extractErr(e, '加载失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  const loadHosts = useCallback(async () => {
    try {
      const { data } = await api.get('/ups/hosts')
      setHosts(data?.data ?? [])
    } catch (e) {
      toast.error(extractErr(e, '加载机器失败'))
    }
  }, [])

  const loadCredentials = useCallback(async () => {
    try {
      const { data } = await api.get('/ups/credentials')
      setCredentials(data?.data ?? [])
    } catch (e) {
      toast.error(extractErr(e, '加载凭证失败'))
    }
  }, [])

  const openHostsDrawer = useCallback(() => {
    setHostsOpen(true)
    void loadHosts()
    void loadCredentials()
  }, [loadHosts, loadCredentials])

  const openCredsDrawer = useCallback(() => {
    setCredsOpen(true)
    void loadCredentials()
  }, [loadCredentials])

  const onAddHost = () => {
    setEditingHost(null)
    setHostEditOpen(true)
  }
  const onEditHost = (h: UpsHost) => {
    setEditingHost(h)
    setHostEditOpen(true)
  }
  const onDeleteHost = async (h: UpsHost) => {
    if (!window.confirm(`确认删除 UPS 机器「${h.name}」?`)) return
    try {
      await api.delete(`/ups/hosts/${h.id}`)
      toast.success('已删除')
      void loadHosts()
      void load()
    } catch (e) {
      toast.error(extractErr(e, '删除失败'))
    }
  }
  const onTestHost = async (h: UpsHost) => {
    try {
      const { data } = await api.post(`/ups/hosts/${h.id}/test`)
      const r = data?.data
      if (r?.ok) {
        const list = ((r.ups_names ?? []) as string[]).filter(Boolean)
        if (list.length > 0) {
          const label = list.length === 1 ? list[0] : `${list.length} 台(${list.join(', ')})`
          toast.success(`连通成功,已识别到 UPS:${label}`)
        } else {
          const diag = (r.diag as string) || ''
          toast.error(diag ? `SSH 已连通,但未拿到 UPS:${diag}` : 'SSH 已连通,但未发现 UPS')
        }
      } else {
        toast.error(r?.error || '连通失败')
      }
    } catch (e) {
      toast.error(extractErr(e, '测试失败'))
    }
  }

  const onAddCredential = () => {
    setEditingCred(null)
    setCredEditOpen(true)
  }
  const onEditCredential = (c: UpsCredential) => {
    setEditingCred(c)
    setCredEditOpen(true)
  }
  const onDeleteCredential = async (c: UpsCredential) => {
    if (!window.confirm(`确认删除 UPS 凭证「${c.name}」?`)) return
    try {
      await api.delete(`/ups/credentials/${c.id}`)
      toast.success('已删除')
      void loadCredentials()
    } catch (e) {
      toast.error(extractErr(e, '删除失败'))
    }
  }

  const triggerSample = useCallback(async () => {
    setRefreshing(true)
    try {
      const { data } = await api.post('/ups/refresh')
      setSnapshots(normalize(data?.data))
      toast.success('已触发一次采样')
    } catch (e) {
      toast.error(extractErr(e, '采样失败'))
    } finally {
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    queueMicrotask(() => {
      void load()
    })
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
    const hosts = snapshots.length
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
            <span
              className={cn('h-2 w-2 cursor-pointer rounded-full', cs.dot)}
              onClick={bumpDemoTap}
              aria-hidden
            />
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
            onClick={openHostsDrawer}
          >
            <Settings2 className="mr-1.5 h-3.5 w-3.5" />
            UPS 机器
          </Button>
        </div>
      </div>

      {!loading && !empty && stats.upses >= 2 && <SummaryCard snapshots={snapshots} />}

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
          <div className="text-[14px] font-medium">还没有 UPS 机器</div>
          <p className="mx-auto max-w-md text-[12.5px] text-muted-foreground">
            点右上「UPS 机器」新增要采样的目标。机器需先在远端装好 NUT(
            <code className="rounded bg-muted px-1">nut-client</code>) +
            <code className="ml-1 rounded bg-muted px-1">upsc</code>。
          </p>
          <Button variant="outline" size="sm" onClick={openHostsDrawer}>
            <Settings2 className="mr-1.5 h-3.5 w-3.5" />
            打开 UPS 机器
          </Button>
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-x-6 gap-y-6 lg:grid-cols-2">
          {snapshots.flatMap((s) =>
            s.upses.length === 0
              ? [
                  <div key={`${s.host_kind}-${s.host_id}-empty`} className="space-y-3">
                    <HostHeader host={s} />
                    <HostEmptyCard host={s} />
                  </div>,
                ]
              : s.upses.map((u) => (
                  <div key={`${s.host_kind}-${s.host_id}-${u.name}`} className="space-y-3">
                    <HostHeader host={s} />
                    <UPSCard
                      ups={u}
                      hostKind={s.host_kind}
                      hostID={s.host_id}
                    />
                  </div>
                )),
          )}
        </div>
      )}

      {demoMode && <DemoSection onClose={() => setDemoMode(false)} />}

      <UpsHostsDrawer
        open={hostsOpen}
        onOpenChange={setHostsOpen}
        hosts={hosts}
        onAdd={onAddHost}
        onEdit={onEditHost}
        onDelete={onDeleteHost}
        onTest={onTestHost}
        onManageCredentials={openCredsDrawer}
      />
      <UpsHostEditDialog
        open={hostEditOpen}
        onOpenChange={setHostEditOpen}
        target={editingHost}
        hosts={hosts}
        credentials={credentials}
        onManageCredentials={openCredsDrawer}
        onSaved={() => {
          void loadHosts()
          void load()
        }}
      />
      <UpsCredentialsDrawer
        open={credsOpen}
        onOpenChange={setCredsOpen}
        credentials={credentials}
        onAdd={onAddCredential}
        onEdit={onEditCredential}
        onDelete={onDeleteCredential}
      />
      <UpsCredentialEditDialog
        open={credEditOpen}
        onOpenChange={setCredEditOpen}
        target={editingCred}
        onSaved={loadCredentials}
      />
    </div>
  )
}
