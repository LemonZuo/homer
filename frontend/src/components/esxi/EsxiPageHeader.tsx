import { AlertTriangle, Loader2, RefreshCw, Settings2 } from 'lucide-react'

import { getColorSet } from '../../colors'
import { cn } from '../../lib/utils'
import { Button } from '../ui/button'
import { fmtDateTime } from './format'

interface EsxiPageHeaderProps {
  stats: {
    hostCnt: number
    onlineHosts: number
    totalVMs: number
    runningVMs: number
    cpuPeak: number
  }
  lastSampled: string
  refreshing: boolean
  onRefresh: () => void
  onOpenHosts: () => void
}

export function EsxiPageHeader({
  stats,
  lastSampled,
  refreshing,
  onRefresh,
  onOpenHosts,
}: EsxiPageHeaderProps) {
  const cs = getColorSet('esxi')

  return (
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
        <Button variant="outline" size="sm" className="flex-1 sm:flex-none" onClick={onOpenHosts}>
          <Settings2 className="mr-1.5 h-3.5 w-3.5" />
          ESXi 机器
        </Button>
      </div>
    </div>
  )
}
