import { Loader2, RefreshCw, Smartphone } from 'lucide-react'
import { Button } from '../ui/button'
import { Card } from '../ui/card'
import { DeviceInfo } from './DeviceInfo'

interface DevicePanelProps {
  config: any
  loading: boolean
  disabled: boolean
  onRefresh: () => void
}

export function DevicePanel({ config, loading, disabled, onRefresh }: DevicePanelProps) {
  return (
    <Card className="mb-4 px-4 py-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Smartphone className="h-4 w-4 text-muted-foreground" />
          <div className="text-[13px] font-medium">设备信息</div>
        </div>
        <Button size="sm" variant="outline" onClick={onRefresh} disabled={loading || disabled}>
          {loading ? (
            <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
          ) : (
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
          )}
          刷新
        </Button>
      </div>
      {config ? (
        <DeviceInfo config={config} />
      ) : (
        <p className="rounded-md border border-dashed border-border py-6 text-center text-[12px] text-muted-foreground">
          点击「刷新」查询设备状态
        </p>
      )}
    </Card>
  )
}
