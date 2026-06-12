import { HardDrive } from 'lucide-react'

import { cn } from '../../../lib/utils'
import { Card } from '../../ui/card'
import { diskStatusPill, diskUsageInfo, fmtBytes, fmtBytesWithZero } from '../format'
import type { DiskHealth } from '../types'
import { SectionHead } from '../ui'

export function DisksCard({ disks }: { disks: DiskHealth[] }) {
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<HardDrive className="h-3.5 w-3.5" />}
        title="磁盘"
        suffix={<span className="text-[11px] text-muted-foreground">{disks.length} 块</span>}
      />
      {disks.length === 0 ? (
        <p className="py-2 text-center text-[11.5px] text-muted-foreground">未拿到 SMART 数据</p>
      ) : (
        <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
          {disks.map((d) => {
            const p = diskStatusPill(d.status)
            const usage = diskUsageInfo(d)
            const datastores = d.datastores ?? []
            return (
              <div
                key={d.device}
                className="rounded-md border border-border/60 bg-muted/30 px-2 py-1.5"
              >
                <div className="flex items-center gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <span className="truncate text-[12px] font-medium text-foreground" title={d.model || d.device}>
                        {d.model || '(无型号)'}
                      </span>
                      {d.type && (
                        <span className="shrink-0 rounded bg-muted px-1 py-0.5 font-mono text-[10px] text-muted-foreground">
                          {d.type}
                        </span>
                      )}
                    </div>
                  </div>
                  <span className="shrink-0 text-[13px] font-semibold tabular-nums text-foreground">
                    {d.temp_c >= 0 ? `${d.temp_c}°C` : '—'}
                  </span>
                  <span className={cn('shrink-0 rounded-full border px-1.5 py-0.5 text-[10.5px] font-medium', p.cls)}>
                    {p.label}
                  </span>
                </div>
                <div className="mt-1.5 flex items-center gap-2">
                  <div className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-muted">
                    {usage.pct !== null && (
                      <div
                        className="h-full rounded-full bg-sky-500 dark:bg-sky-400"
                        style={{ width: `${usage.pct}%` }}
                      />
                    )}
                  </div>
                  <span
                    className="shrink-0 text-[10.5px] tabular-nums text-muted-foreground"
                    title={
                      usage.pct !== null
                        ? `已用 ${fmtBytesWithZero(usage.used)} / 总 ${fmtBytes(usage.capacity > 0 ? usage.capacity : usage.used + Math.max(0, usage.free))}`
                        : usage.capacity > 0
                          ? `总 ${fmtBytes(usage.capacity)}`
                          : undefined
                    }
                  >
                    {usage.label}
                  </span>
                </div>
                {(() => {
                  const hours = d.smart_power_on_hours ?? -1
                  const cycles = d.smart_power_cycle_count ?? -1
                  const wear = d.smart_media_wearout ?? -1
                  const realloc = d.smart_reallocated_sectors ?? 0
                  const pending = d.smart_pending_sector_realloc ?? 0
                  const uncorr = d.smart_uncorrectable_errors ?? 0
                  const readErr = d.smart_read_error_count ?? 0
                  const health = (d.smart_health ?? '').trim()

                  const facts: string[] = []
                  if (hours >= 0) {
                    facts.push(hours >= 8760 ? `通电 ${(hours / 8760).toFixed(1)}y` : `通电 ${hours}h`)
                  }
                  if (cycles >= 0) facts.push(`开机 ${cycles} 次`)

                  type Pill = { label: string; tone: 'red' | 'amber' | 'green' }
                  const pills: Pill[] = []
                  if (wear >= 0) {
                    const tone: Pill['tone'] = wear >= 80 ? 'green' : wear >= 60 ? 'amber' : 'red'
                    pills.push({ label: `健康 ${wear}%`, tone })
                  }
                  if (realloc > 0) pills.push({ label: `重映射 ${realloc}`, tone: realloc >= 5 ? 'red' : 'amber' })
                  if (pending > 0) pills.push({ label: `待重映射 ${pending}`, tone: 'red' })
                  if (uncorr > 0) pills.push({ label: `不可纠正 ${uncorr}`, tone: 'red' })
                  if (readErr > 0) pills.push({ label: `读错误 ${readErr}`, tone: 'amber' })
                  if (health && health !== 'OK') pills.push({ label: `SMART: ${health}`, tone: 'red' })

                  if (facts.length === 0 && pills.length === 0) return null
                  const pillCls: Record<Pill['tone'], string> = {
                    red: 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300',
                    amber: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
                    green: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
                  }
                  return (
                    <div className="mt-1 flex flex-wrap items-center gap-1 text-[10.5px] text-muted-foreground">
                      {facts.length > 0 && <span className="tabular-nums">{facts.join(' · ')}</span>}
                      {pills.map((p, i) => (
                        <span
                          key={i}
                          className={cn('rounded-full border px-1.5 py-0.5 font-medium tabular-nums', pillCls[p.tone])}
                        >
                          {p.label}
                        </span>
                      ))}
                    </div>
                  )
                })()}
                {datastores.length > 0 && (
                  <div
                    className="mt-1 truncate text-[10.5px] text-muted-foreground"
                    title={datastores.join(', ')}
                  >
                    {datastores.join(', ')}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </Card>
  )
}
