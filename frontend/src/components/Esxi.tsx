import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  RefreshCw,
  Loader2,
  Settings2,
  AlertTriangle,
  Server,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { cn } from '../lib/utils'
import { EsxiHostsDrawer } from './esxi/EsxiHostsDrawer'
import { EsxiHostEditDialog } from './esxi/EsxiHostEditDialog'
import { EsxiCredentialsDrawer } from './esxi/EsxiCredentialsDrawer'
import { EsxiCredentialEditDialog } from './esxi/EsxiCredentialEditDialog'
import { HostBlock } from './esxi/HostBlock'
import { extractErr, fmtDateTime, isStaleSample, useNowTick } from './esxi/format'
import type { EsxiCredential, EsxiHost, Snapshot } from './esxi/types'

// --- 主页面 ---

export default function Esxi() {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [hostsOpen, setHostsOpen] = useState(false)
  const [hostEditOpen, setHostEditOpen] = useState(false)
  const [editingHost, setEditingHost] = useState<EsxiHost | null>(null)
  const [credsOpen, setCredsOpen] = useState(false)
  const [credEditOpen, setCredEditOpen] = useState(false)
  const [editingCred, setEditingCred] = useState<EsxiCredential | null>(null)
  const [hosts, setHosts] = useState<EsxiHost[]>([])
  const [credentials, setCredentials] = useState<EsxiCredential[]>([])

  const normalize = (arr: unknown): Snapshot[] => {
    if (!Array.isArray(arr)) return []
    return arr as Snapshot[]
  }

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/esxi/snapshot')
      setSnapshots(normalize(data?.data))
    } catch (e) {
      toast.error(extractErr(e, '加载失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  const loadHosts = useCallback(async () => {
    try {
      const { data } = await api.get('/esxi/hosts')
      setHosts(data?.data ?? [])
    } catch (e) {
      toast.error(extractErr(e, '加载机器失败'))
    }
  }, [])

  const loadCredentials = useCallback(async () => {
    try {
      const { data } = await api.get('/esxi/credentials')
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
  const onEditHost = (h: EsxiHost) => {
    setEditingHost(h)
    setHostEditOpen(true)
  }
  const onDeleteHost = async (h: EsxiHost) => {
    if (!window.confirm(`确认删除 ESXi 机器「${h.name}」?`)) return
    try {
      await api.delete(`/esxi/hosts/${h.id}`)
      toast.success('已删除')
      void loadHosts()
      void load()
    } catch (e) {
      toast.error(extractErr(e, '删除失败'))
    }
  }
  const onTestHost = async (h: EsxiHost) => {
    try {
      const { data } = await api.post(`/esxi/hosts/${h.id}/test`)
      const r = data?.data
      if (r?.ok) {
        toast.success(r.summary ? `连通成功 · ${r.summary}` : '连通成功')
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
  const onEditCredential = (c: EsxiCredential) => {
    setEditingCred(c)
    setCredEditOpen(true)
  }
  const onDeleteCredential = async (c: EsxiCredential) => {
    if (!window.confirm(`确认删除 ESXi 凭证「${c.name}」?`)) return
    try {
      await api.delete(`/esxi/credentials/${c.id}`)
      toast.success('已删除')
      void loadCredentials()
    } catch (e) {
      toast.error(extractErr(e, '删除失败'))
    }
  }

  const triggerSample = useCallback(async () => {
    setRefreshing(true)
    try {
      const { data } = await api.post('/esxi/refresh')
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

  useEffect(() => {
    const es = new EventSource('/api/esxi/stream')
    es.addEventListener('snapshot', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data)
        setSnapshots(normalize(data))
        setLoading(false)
      } catch {
        // 单帧损坏忽略
      }
    })
    return () => es.close()
  }, [])

  const cs = getColorSet('esxi')
  const empty = !loading && snapshots.length === 0
  const now = useNowTick()

  const stats = useMemo(() => {
    const hostCnt = snapshots.length
    let onlineHosts = 0
    let totalVMs = 0
    let runningVMs = 0
    let cpuPeak = -1
    for (const s of snapshots) {
      if (s.reachable && !isStaleSample(s.sampled_at, now)) onlineHosts++
      if (s.vms) {
        totalVMs += s.vms.length
        for (const v of s.vms) {
          if (v.state === 'powered_on') runningVMs++
        }
      }
      if (s.cpu_temperature?.max_c != null && s.cpu_temperature.max_c > cpuPeak) {
        cpuPeak = s.cpu_temperature.max_c
      }
    }
    return { hostCnt, onlineHosts, totalVMs, runningVMs, cpuPeak }
  }, [now, snapshots])

  const lastSampled = useMemo(() => {
    let latest = ''
    for (const s of snapshots) {
      if (s.sampled_at && (!latest || s.sampled_at > latest)) latest = s.sampled_at
    }
    return latest
  }, [snapshots])

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="hidden sm:block">
          <div className="flex items-center gap-3">
            <span className={cn('h-2 w-2 rounded-full', cs.dot)} aria-hidden />
            <h1 className="text-[28px] font-bold leading-none tracking-tight">ESXi 状态</h1>
          </div>
          {stats.hostCnt > 1 && (
            <p className="mt-2 text-[12.5px] text-muted-foreground">
              {stats.hostCnt} 台机器
              {stats.onlineHosts < stats.hostCnt && (
                <span className="ml-2 inline-flex items-center gap-1 text-amber-600 dark:text-amber-400">
                  <AlertTriangle className="h-3 w-3" />
                  {stats.hostCnt - stats.onlineHosts} 台离线
                </span>
              )}
              {stats.totalVMs > 0 && (
                <span className="ml-2">· {stats.runningVMs} / {stats.totalVMs} VM 运行中</span>
              )}
              {stats.cpuPeak >= 0 && (
                <span className="ml-2">· CPU 峰值 {stats.cpuPeak}°C</span>
              )}
              {lastSampled && (
                <span className="ml-2 text-muted-foreground/70">· 最近采样 {fmtDateTime(lastSampled)}</span>
              )}
            </p>
          )}
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
          <Button variant="outline" size="sm" className="flex-1 sm:flex-none" onClick={openHostsDrawer}>
            <Settings2 className="mr-1.5 h-3.5 w-3.5" />
            ESXi 机器
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
            <Server className="h-5 w-5 text-muted-foreground" />
          </div>
          <div className="text-[14px] font-medium">还没有 ESXi 机器</div>
          <p className="mx-auto max-w-md text-[12.5px] text-muted-foreground">
            点右上「ESXi 机器」新增要采样的主机;需先开放 ESXi 的 SSH(默认是关闭的)。
          </p>
          <Button variant="outline" size="sm" onClick={openHostsDrawer}>
            <Settings2 className="mr-1.5 h-3.5 w-3.5" />
            打开 ESXi 机器
          </Button>
        </Card>
      ) : (
        <div className="space-y-6">
          {snapshots.map((s) => (
            <HostBlock key={`${s.host_kind}-${s.host_id}`} host={s} />
          ))}
        </div>
      )}

      <EsxiHostsDrawer
        open={hostsOpen}
        onOpenChange={setHostsOpen}
        hosts={hosts}
        onAdd={onAddHost}
        onEdit={onEditHost}
        onDelete={onDeleteHost}
        onTest={onTestHost}
        onManageCredentials={openCredsDrawer}
      />
      <EsxiHostEditDialog
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
      <EsxiCredentialsDrawer
        open={credsOpen}
        onOpenChange={setCredsOpen}
        credentials={credentials}
        onAdd={onAddCredential}
        onEdit={onEditCredential}
        onDelete={onDeleteCredential}
      />
      <EsxiCredentialEditDialog
        open={credEditOpen}
        onOpenChange={setCredEditOpen}
        target={editingCred}
        onSaved={loadCredentials}
      />
    </div>
  )
}
