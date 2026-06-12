import { useCallback, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Card } from './ui/card'
import { UpsHostsDrawer } from './ups/UpsHostsDrawer'
import { UpsHostEditDialog } from './ups/UpsHostEditDialog'
import { UpsCredentialsDrawer } from './ups/UpsCredentialsDrawer'
import { UpsCredentialEditDialog } from './ups/UpsCredentialEditDialog'
import { DemoSection } from './ups/DemoSection'
import { HostSection } from './ups/HostSection'
import { SummaryCard } from './ups/SummaryCard'
import { UpsEmptyState } from './ups/UpsEmptyState'
import { UpsPageHeader } from './ups/UpsPageHeader'
import { useUpsManagement } from './ups/useUpsManagement'
import { useUpsSnapshots } from './ups/useUpsSnapshots'

export default function Ups() {
  const {
    snapshots,
    loading,
    refreshing,
    empty,
    stats,
    lastSampled,
    reload: reloadSnapshots,
    triggerSample,
  } = useUpsSnapshots()
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
  } = useUpsManagement({ reloadSnapshots })
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

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <UpsPageHeader
        stats={stats}
        lastSampled={lastSampled}
        refreshing={refreshing}
        onRefresh={triggerSample}
        onOpenHosts={openHostsDrawer}
        onDemoTap={bumpDemoTap}
      />

      {!loading && !empty && stats.upses >= 2 && <SummaryCard snapshots={snapshots} />}

      {loading ? (
        <Card className="px-4 py-16 text-center text-[12.5px] text-muted-foreground">
          <Loader2 className="mx-auto mb-2 h-4 w-4 animate-spin" />
          加载中
        </Card>
      ) : empty ? (
        <UpsEmptyState onOpenHosts={openHostsDrawer} />
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
          void reloadSnapshots()
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
