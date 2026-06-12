import {
  RefreshCw,
  Loader2,
  Settings2,
  AlertTriangle,
  Server,
} from 'lucide-react'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { cn } from '../lib/utils'
import { EsxiHostsDrawer } from './esxi/EsxiHostsDrawer'
import { EsxiHostEditDialog } from './esxi/EsxiHostEditDialog'
import { EsxiCredentialsDrawer } from './esxi/EsxiCredentialsDrawer'
import { EsxiCredentialEditDialog } from './esxi/EsxiCredentialEditDialog'
import { HostBlock } from './esxi/HostBlock'
import { fmtDateTime } from './esxi/format'
import { useEsxiManagement } from './esxi/useEsxiManagement'
import { useEsxiSnapshots } from './esxi/useEsxiSnapshots'

// --- 主页面 ---

export default function Esxi() {
  const {
    snapshots,
    loading,
    refreshing,
    empty,
    stats,
    lastSampled,
    reload: reloadSnapshots,
    triggerSample,
  } = useEsxiSnapshots()
  const {
    hostsOpen,
    setHostsOpen,
    hostEditOpen,
    setHostEditOpen,
    editingHost,
    credsOpen,
    setCredsOpen,
    credEditOpen,
    setCredEditOpen,
    editingCred,
    hosts,
    credentials,
    loadHosts,
    loadCredentials,
    openHostsDrawer,
    openCredsDrawer,
    onAddHost,
    onEditHost,
    onDeleteHost,
    onTestHost,
    onAddCredential,
    onEditCredential,
    onDeleteCredential,
  } = useEsxiManagement({ reloadSnapshots })

  const cs = getColorSet('esxi')

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
          void reloadSnapshots()
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
