import { Loader2 } from 'lucide-react'
import { Card } from './ui/card'
import { EsxiEmptyState } from './esxi/EsxiEmptyState'
import { EsxiManagementDialogs } from './esxi/EsxiManagementDialogs'
import { EsxiPageHeader } from './esxi/EsxiPageHeader'
import { HostBlock } from './esxi/HostBlock'
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
  const management = useEsxiManagement({ reloadSnapshots })

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <EsxiPageHeader
        stats={stats}
        lastSampled={lastSampled}
        refreshing={refreshing}
        onRefresh={triggerSample}
        onOpenHosts={management.openHostsDrawer}
      />

      {loading ? (
        <Card className="px-4 py-16 text-center text-[12.5px] text-muted-foreground">
          <Loader2 className="mx-auto mb-2 h-4 w-4 animate-spin" />
          加载中
        </Card>
      ) : empty ? (
        <EsxiEmptyState onOpenHosts={management.openHostsDrawer} />
      ) : (
        <div className="space-y-6">
          {snapshots.map((s) => (
            <HostBlock key={`${s.host_kind}-${s.host_id}`} host={s} />
          ))}
        </div>
      )}

      <EsxiManagementDialogs management={management} onHostSaved={reloadSnapshots} />
    </div>
  )
}
