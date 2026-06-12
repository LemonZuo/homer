import { useMemo } from 'react'

import { cn } from '../../lib/utils'
import { Card } from '../ui/card'
import { fmtRuntime, isStaleSample, useNowTick } from './format'
import type { Snapshot } from './types'

export function SummaryCard({ snapshots }: { snapshots: Snapshot[] }) {
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
