import { useCallback, useState } from 'react'
import {
  CheckCircle2,
  ChevronDown,
  History,
  Loader2,
  MinusCircle,
  Play,
  XCircle,
} from 'lucide-react'
import { getColorSet } from '../../colors'
import { cn } from '../../lib/utils'
import { Button } from '../ui/button'
import { Card } from '../ui/card'
import { describeCron, fmtTime, triggerLabel } from './format'
import { DEFAULT_JOB_META, JOB_META } from './jobMeta'
import { RunLine } from './RunLine'
import { STATUS_STYLE, statusOf } from './status'
import type { Job } from './types'

interface JobCardProps {
  job: Job
  accent: ReturnType<typeof getColorSet>
  onRun: (name: string) => Promise<void>
}

export function JobCard({ job, accent, onRun }: JobCardProps) {
  const [expanded, setExpanded] = useState(false)
  const [running, setRunning] = useState(false)

  const trigger = useCallback(async () => {
    setRunning(true)
    try {
      await onRun(job.name)
    } finally {
      setRunning(false)
    }
  }, [job.name, onRun])

  const meta = JOB_META[job.name] ?? DEFAULT_JOB_META
  const Icon = meta.icon
  const status = statusOf(job)
  const st = STATUS_STYLE[status]

  return (
    <Card
      className={cn(
        'overflow-hidden border bg-card p-0 transition-all',
        accent.border,
        accent.halo,
      )}
    >
      <div className="p-3.5 sm:p-4">
        <div className="flex items-center gap-3">
          <div
            className={cn(
              'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg',
              accent.picker,
            )}
          >
            <Icon className="h-[18px] w-[18px]" />
          </div>

          <div className="flex min-w-0 flex-1 items-center gap-x-2 gap-y-0.5">
            <span className="truncate text-[14px] font-semibold leading-tight sm:text-[15px]">
              {job.name}
            </span>
            {meta.label && (
              <span className="shrink-0 text-[12px] text-muted-foreground">
                {meta.label}
              </span>
            )}
            <span
              className={cn(
                'flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium',
                st.chip,
              )}
            >
              <span className={cn('h-1.5 w-1.5 rounded-full', st.dot)} />
              {st.label}
            </span>
          </div>

          <Button
            size="sm"
            variant="outline"
            onClick={trigger}
            disabled={running || job.running}
            className="h-8 shrink-0 px-3"
          >
            {running || job.running ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin sm:mr-1.5" />
            ) : (
              <Play className="h-3.5 w-3.5 sm:mr-1.5" />
            )}
            <span className="hidden sm:inline">立即执行</span>
          </Button>
        </div>

        <div className="mt-3 grid grid-cols-1 gap-x-6 gap-y-1.5 sm:grid-cols-3 sm:gap-y-0">
          <div className="flex items-center justify-between gap-3 sm:flex-col sm:items-start sm:gap-1">
            <span className="text-[11px] uppercase tracking-wide text-muted-foreground/70">
              周期
            </span>
            <span className="flex items-center gap-1.5 text-[12px]">
              {job.manual_only ? (
                <span className="text-muted-foreground">仅手动</span>
              ) : (
                <>
                  <span className="text-foreground/85">
                    {describeCron(job.spec) || '自定义'}
                  </span>
                  <code
                    className="rounded bg-muted px-1 py-0.5 font-mono text-[10.5px] text-muted-foreground"
                    title={job.spec}
                  >
                    {job.spec}
                  </code>
                </>
              )}
            </span>
          </div>

          <div className="flex items-center justify-between gap-3 sm:flex-col sm:items-start sm:gap-1">
            <span className="text-[11px] uppercase tracking-wide text-muted-foreground/70">
              下次执行
            </span>
            <span className="font-mono text-[12px] text-foreground/85">
              {job.manual_only ? '—' : fmtTime(job.next)}
            </span>
          </div>

          <div className="flex items-center justify-between gap-3 sm:flex-col sm:items-start sm:gap-1">
            <span className="text-[11px] uppercase tracking-wide text-muted-foreground/70">
              上次执行
            </span>
            {job.last ? (
              <span className={cn('flex items-center gap-1.5 text-[12px]', st.text)}>
                {job.last.skipped ? (
                  <MinusCircle className="h-3.5 w-3.5 shrink-0" />
                ) : job.last.ok ? (
                  <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
                ) : (
                  <XCircle className="h-3.5 w-3.5 shrink-0" />
                )}
                <span className="font-mono">{fmtTime(job.last.start)}</span>
                <span className="text-muted-foreground">{triggerLabel(job.last.trigger)}</span>
              </span>
            ) : (
              <span className="text-[12px] text-muted-foreground">—</span>
            )}
          </div>
        </div>

        {job.last && job.last.skipped && job.last.err && (
          <div className="mt-2.5 break-words rounded-md bg-amber-500/5 px-2.5 py-1.5 font-mono text-[11.5px] text-amber-600 dark:text-amber-400">
            {job.last.err}
          </div>
        )}
        {job.last && !job.last.skipped && !job.last.ok && job.last.err && (
          <div className="mt-2.5 break-words rounded-md bg-rose-500/5 px-2.5 py-1.5 font-mono text-[11.5px] text-rose-600 dark:text-rose-400">
            {job.last.err}
          </div>
        )}
      </div>

      {job.history.length > 0 && (
        <div className="border-t border-border/60 bg-muted/30 px-4 sm:px-5">
          <button
            type="button"
            onClick={() => setExpanded((v) => !v)}
            className="flex w-full items-center gap-1.5 py-2.5 text-[12px] font-medium text-muted-foreground transition-colors hover:text-foreground"
          >
            <History className="h-3.5 w-3.5" />
            最近 {job.history.length} 次执行
            <ChevronDown
              className={cn('h-3.5 w-3.5 transition-transform', expanded && 'rotate-180')}
            />
          </button>
          {expanded && (
            <div className="divide-y divide-border/50 pb-2">
              {job.history.map((run, index) => (
                <RunLine key={index} run={run} />
              ))}
            </div>
          )}
        </div>
      )}
    </Card>
  )
}
