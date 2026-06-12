import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  RefreshCw,
  Loader2,
  Settings2,
  Plug,
  AlertTriangle,
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
import { extractErr, fmtDateTime } from './ups/format'
import type { Snapshot, SnapshotUPS, UpsCredential, UpsHost } from './ups/types'
import { DemoSection } from './ups/DemoSection'
import { HostSection } from './ups/HostSection'
import { SummaryCard } from './ups/SummaryCard'

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
          {snapshots.map((s) => (
            <HostSection key={`${s.host_kind}-${s.host_id}`} host={s} />
          ))}
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
