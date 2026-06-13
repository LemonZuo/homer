import { Timer } from 'lucide-react'
import { cn } from '../../lib/utils'
import { fmtDuration, fmtTime, triggerLabel } from './format'
import type { Run } from './types'

export function RunLine({ run }: { run: Run }) {
  const dotColor = run.skipped ? 'bg-amber-500' : run.ok ? 'bg-emerald-500' : 'bg-rose-500'
  return (
    <div className="flex items-start gap-2.5 py-2">
      <span className={cn('mt-1 h-1.5 w-1.5 shrink-0 rounded-full', dotColor)} />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[12px]">
          <span className="font-mono text-foreground/80">{fmtTime(run.start)}</span>
          <span className="rounded bg-muted px-1.5 py-0.5 text-[10.5px] font-medium text-muted-foreground">
            {triggerLabel(run.trigger)}
          </span>
          {run.skipped && (
            <span className="rounded bg-amber-500/10 px-1.5 py-0.5 text-[10.5px] font-medium text-amber-600 dark:text-amber-400">
              跳过
            </span>
          )}
          <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
            <Timer className="h-3 w-3" />
            {fmtDuration(run.start, run.end)}
          </span>
        </div>
        {run.skipped && run.err && (
          <div className="mt-1 break-words rounded-md bg-amber-500/5 px-2 py-1 font-mono text-[11.5px] text-amber-600 dark:text-amber-400">
            {run.err}
          </div>
        )}
        {!run.skipped && !run.ok && run.err && (
          <div className="mt-1 break-words rounded-md bg-rose-500/5 px-2 py-1 font-mono text-[11.5px] text-rose-600 dark:text-rose-400">
            {run.err}
          </div>
        )}
      </div>
    </div>
  )
}
