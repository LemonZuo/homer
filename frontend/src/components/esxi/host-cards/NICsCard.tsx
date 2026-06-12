import { Network } from 'lucide-react'

import { cn } from '../../../lib/utils'
import { Card } from '../../ui/card'
import { fmtBitrate, fmtBytes } from '../format'
import type { NIC } from '../types'
import { SectionHead } from '../ui'

export function NICsCard({ nics }: { nics: NIC[] }) {
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Network className="h-3.5 w-3.5" />}
        title="网卡"
        suffix={
          <span className="rounded-md border border-border bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">
            {nics.length} 张
          </span>
        }
      />
      <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
        {nics.map((n) => {
          const linkUp = n.link_status === 'Up'
          const adminUp = n.admin_status === 'Up'
          const linkTone = linkUp
            ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
            : 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300'
          type Pill = { label: string; tone: 'red' | 'amber' }
          const pills: Pill[] = []
          if (n.rx_errors > 0) pills.push({ label: `收错 ${n.rx_errors}`, tone: 'red' })
          if (n.tx_errors > 0) pills.push({ label: `发错 ${n.tx_errors}`, tone: 'red' })
          if (n.rx_dropped > 0) pills.push({ label: `收丢 ${n.rx_dropped}`, tone: 'amber' })
          if (n.tx_dropped > 0) pills.push({ label: `发丢 ${n.tx_dropped}`, tone: 'amber' })
          const pillCls: Record<Pill['tone'], string> = {
            red: 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300',
            amber: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
          }
          return (
            <div key={n.name} className="rounded-md border border-border bg-muted/30 px-2.5 py-2">
              <div className="flex items-start gap-2">
                <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
                  <span className="font-mono text-[12px] font-medium text-foreground">{n.name}</span>
                  <span className={cn('rounded-md border px-1.5 py-0.5 text-[10.5px] font-medium', linkTone)}>
                    {linkUp ? '链路 Up' : '链路 Down'}
                  </span>
                  {linkUp ? (
                    <span className="rounded-md border border-border bg-background px-1.5 py-0.5 text-[10.5px] text-muted-foreground">
                      {fmtBitrate(n.speed_mbps)}
                      {n.duplex ? ` · ${n.duplex}` : ''}
                    </span>
                  ) : null}
                  {!adminUp ? (
                    <span className="rounded-md border border-amber-500/40 bg-amber-500/10 px-1.5 py-0.5 text-[10.5px] text-amber-700 dark:text-amber-300">
                      Admin Down
                    </span>
                  ) : null}
                  {pills.map((p) => (
                    <span
                      key={p.label}
                      className={cn('rounded-md border px-1.5 py-0.5 text-[10.5px]', pillCls[p.tone])}
                    >
                      {p.label}
                    </span>
                  ))}
                </div>
                {n.description ? (
                  <span
                    className="min-w-0 max-w-[55%] truncate text-right text-[11px] text-muted-foreground"
                    title={n.description}
                  >
                    {n.description}
                  </span>
                ) : null}
              </div>
              <div className="mt-1.5 grid grid-cols-2 gap-x-3 gap-y-0.5 text-[11px] text-muted-foreground">
                <span>MAC <span className="font-mono text-foreground">{n.mac || '—'}</span></span>
                <span>驱动 <span className="text-foreground">{n.driver || '—'}</span></span>
                <span>收 <span className="text-foreground">{n.rx_bytes >= 0 ? fmtBytes(n.rx_bytes) : '—'}</span></span>
                <span>发 <span className="text-foreground">{n.tx_bytes >= 0 ? fmtBytes(n.tx_bytes) : '—'}</span></span>
              </div>
            </div>
          )
        })}
      </div>
    </Card>
  )
}
