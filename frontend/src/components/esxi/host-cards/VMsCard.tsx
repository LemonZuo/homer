import { Box } from 'lucide-react'

import { cn } from '../../../lib/utils'
import { Card } from '../../ui/card'
import { vmStatePill } from '../format'
import type { VM } from '../types'
import { SectionHead } from '../ui'

export function VMsCard({ vms }: { vms: VM[] }) {
  const on = vms.filter((v) => v.state === 'powered_on').length
  const sorted = [...vms].sort((a, b) => a.id - b.id)
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Box className="h-3.5 w-3.5" />}
        title="虚拟机"
        suffix={
          <span className="text-[11px] tabular-nums text-muted-foreground">
            <span className="font-semibold text-emerald-600 dark:text-emerald-400">{on}</span>
            <span className="mx-0.5">/</span>
            <span>{vms.length}</span>
            <span className="ml-1">运行中</span>
          </span>
        }
      />
      {vms.length === 0 ? (
        <p className="py-2 text-center text-[11.5px] text-muted-foreground">没有虚拟机</p>
      ) : (
        <div className="grid grid-cols-1 gap-1 md:grid-cols-2">
          {sorted.map((v) => {
            const p = vmStatePill(v.state)
            return (
              <div key={v.id} className="flex items-center gap-2 rounded-md border border-border/60 bg-muted/30 px-2 py-1.5">
                <span className="shrink-0 rounded bg-muted px-1 py-0.5 font-mono text-[10px] text-muted-foreground">
                  #{v.id}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-[12px] font-medium text-foreground" title={v.name}>
                    {v.name}
                  </div>
                </div>
                <span className={cn('shrink-0 rounded-full border px-2 py-0.5 text-[10.5px] font-medium', p.cls)}>
                  <span className="inline-flex items-center gap-1">
                    <span className={cn('h-1.5 w-1.5 rounded-full', p.dot)} />
                    {p.label}
                  </span>
                </span>
              </div>
            )
          })}
        </div>
      )}
    </Card>
  )
}
