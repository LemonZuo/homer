import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  RefreshCw,
  Loader2,
  Settings2,
  Plug,
  AlertTriangle,
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
import { DEMO_BATTERY_VARIANTS } from './ups/constants'
import { extractErr, fmtDateTime, fmtRuntime, isStaleSample, useNowTick } from './ups/format'
import type { Snapshot, SnapshotUPS, UpsCredential, UpsHost } from './ups/types'
import { UPSCard } from './ups/UPSCard'

// 垂直电池:≤20% rose / ≤50% amber / 否则 emerald;飞牛风方形圆角 + 一条清晰白光。
// 动效按 power_source 区分:mains 走上升扫描光(在线/充电感),battery/low_battery
// 走整体呼吸 opacity(正在被消耗),unknown / 离线无动效。
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
