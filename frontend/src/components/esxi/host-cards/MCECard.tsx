import { ShieldAlert, ShieldCheck } from 'lucide-react'

import { cn } from '../../../lib/utils'
import { Card } from '../../ui/card'
import type { MCEHealth } from '../types'
import { KV, SectionHead } from '../ui'

export function MCECard({ m }: { m: MCEHealth }) {
  const state = (m.state || '').toLowerCase()
  const tone =
    state === 'green'
      ? { pill: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300', dot: 'bg-emerald-500', icon: ShieldCheck, label: '健康' }
      : state === 'yellow'
        ? { pill: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300', dot: 'bg-amber-500', icon: ShieldAlert, label: '警告' }
        : state === 'red'
          ? { pill: 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300', dot: 'bg-rose-500', icon: ShieldAlert, label: '危险' }
          : { pill: 'border-border bg-muted text-muted-foreground', dot: 'bg-muted-foreground/60', icon: ShieldCheck, label: m.state || '未知' }
  const Icon = tone.icon
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Icon className="h-3.5 w-3.5" />}
        title="MCE"
        suffix={
          <span className={cn('rounded-full border px-2 py-0.5 text-[11px] font-medium', tone.pill)}>
            <span className="inline-flex items-center gap-1.5">
              <span className={cn('h-1.5 w-1.5 rounded-full', tone.dot)} />
              {tone.label}
            </span>
          </span>
        }
      />
      <div className="grid grid-cols-2 gap-2.5">
        <KV k="可纠正错误" v={(m.corrected_total ?? 0).toLocaleString()} />
        <KV k="不可纠正" v={
          <span className={m.uncorrected_total > 0 ? 'text-rose-600 dark:text-rose-400' : undefined}>
            {(m.uncorrected_total ?? 0).toLocaleString()}
          </span>
        } />
        <KV k="EWMA / 周期" v={`${m.corrected_ewma ?? 0} / ${m.period_seconds ?? 0}s`} />
      </div>
    </Card>
  )
}
