import type { Job } from './types'

export type JobStatus = 'running' | 'ok' | 'fail' | 'skipped' | 'idle'

export function statusOf(job: Job): JobStatus {
  if (job.running) return 'running'
  if (!job.last) return 'idle'
  if (job.last.skipped) return 'skipped'
  return job.last.ok ? 'ok' : 'fail'
}

export const STATUS_STYLE: Record<JobStatus, { dot: string; text: string; label: string; chip: string }> = {
  running: {
    dot: 'bg-sky-500 animate-pulse',
    text: 'text-sky-600 dark:text-sky-400',
    label: '执行中',
    chip: 'bg-sky-500/10 text-sky-600 dark:text-sky-400',
  },
  ok: {
    dot: 'bg-emerald-500',
    text: 'text-emerald-600 dark:text-emerald-400',
    label: '上次成功',
    chip: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  },
  fail: {
    dot: 'bg-rose-500',
    text: 'text-rose-600 dark:text-rose-400',
    label: '上次失败',
    chip: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
  },
  skipped: {
    dot: 'bg-amber-500',
    text: 'text-amber-600 dark:text-amber-400',
    label: '上次跳过',
    chip: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  },
  idle: {
    dot: 'bg-muted-foreground/40',
    text: 'text-muted-foreground',
    label: '尚未执行',
    chip: 'bg-muted text-muted-foreground',
  },
}
