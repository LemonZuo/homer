import { AlertTriangle, Loader2, RefreshCw, Settings2 } from 'lucide-react'

import { getColorSet } from '../../colors'
import { cn } from '../../lib/utils'
import { Button } from '../ui/button'
import { fmtDateTime } from './format'

interface UpsPageHeaderProps {
  stats: {
    hosts: number
    upses: number
    alerts: number
  }
  lastSampled: string
  refreshing: boolean
  onRefresh: () => void
  onOpenHosts: () => void
  onDemoTap: () => void
}

export function UpsPageHeader({
  stats,
  lastSampled,
  refreshing,
  onRefresh,
  onOpenHosts,
  onDemoTap,
}: UpsPageHeaderProps) {
  const cs = getColorSet('teal')

  return (
    <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div className="hidden sm:block">
        <div className="flex items-center gap-3">
          <span
            className={cn('h-2 w-2 cursor-pointer rounded-full', cs.dot)}
            onClick={onDemoTap}
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
          onClick={onRefresh}
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
          onClick={onOpenHosts}
        >
          <Settings2 className="mr-1.5 h-3.5 w-3.5" />
          UPS 机器
        </Button>
      </div>
    </div>
  )
}
