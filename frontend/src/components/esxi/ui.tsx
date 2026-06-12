import type React from 'react'

import { cn } from '../../lib/utils'
import { Card } from '../ui/card'

export function SectionHead({ icon, title, suffix }: { icon: React.ReactNode; title: string; suffix?: React.ReactNode }) {
  return (
    <div className="mb-2 flex items-center justify-between gap-2">
      <div className="flex items-center gap-1.5 text-[12px] font-semibold text-foreground/90">
        {icon}
        <span>{title}</span>
      </div>
      {suffix}
    </div>
  )
}

export function EmptyCard({ icon, title, hint = '暂无数据' }: { icon: React.ReactNode; title: string; hint?: string }) {
  return (
    <Card className="px-3 py-3">
      <SectionHead icon={icon} title={title} />
      <div className="mt-3 rounded-md border border-dashed border-border/60 bg-muted/20 py-4 text-center text-[11.5px] text-muted-foreground">
        {hint}
      </div>
    </Card>
  )
}

export function KV({ k, v, mono = false, title }: { k: string; v: React.ReactNode; mono?: boolean; title?: string }) {
  return (
    <div className="min-w-0">
      <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">{k}</div>
      <div
        className={cn(
          'mt-0.5 truncate text-[13px] font-medium text-foreground',
          mono && 'font-mono text-[12px]',
        )}
        title={title}
      >
        {v}
      </div>
    </div>
  )
}
