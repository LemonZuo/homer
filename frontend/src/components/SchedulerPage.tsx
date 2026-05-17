import { useCallback, useEffect, useState } from 'react'
import type { ComponentType } from 'react'
import {
  Cake,
  CalendarClock,
  CheckCircle2,
  ChevronDown,
  Clock,
  History,
  Loader2,
  Play,
  RefreshCw,
  ShieldCheck,
  Timer,
  XCircle,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { cn } from '../lib/utils'

interface Run {
  start: string
  end: string
  ok: boolean
  err?: string
  trigger: string
}

interface Job {
  name: string
  spec: string
  manual_only: boolean
  next?: string
  running: boolean
  last?: Run
  history: Run[]
}

const JOB_META: Record<string, { icon: ComponentType<{ className?: string }>; label: string }> = {
  birthday: { icon: Cake, label: '生日提醒' },
  event: { icon: CalendarClock, label: '事项提醒' },
  'acme-renew': { icon: ShieldCheck, label: 'ACME 续期' },
  'acme-deploy-retry': { icon: RefreshCw, label: '部署失败重试' },
}

function fmtTime(v?: string): string {
  if (!v) return '—'
  const d = new Date(v)
  if (isNaN(d.getTime())) return '—'
  const p = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

function fmtDuration(a: string, b: string): string {
  const ms = new Date(b).getTime() - new Date(a).getTime()
  if (!Number.isFinite(ms) || ms < 0) return ''
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

const triggerLabel = (t: string) => (t === 'manual' ? '手动' : '定时')

const WEEKDAYS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

// 把 6 段 cron（秒 分 时 日 月 周）翻译成中文语义；命中不了就返回空串。
function describeCron(spec: string): string {
  const f = spec.trim().split(/\s+/)
  if (f.length !== 6) return ''
  const [sec, min, hour, dom, mon, dow] = f
  const num = (v: string) => (/^\d+$/.test(v) ? Number(v) : null)
  const s = num(sec)
  const m = num(min)
  const h = num(hour)
  if (s == null || m == null || h == null) return ''
  const p = (x: number) => String(x).padStart(2, '0')
  const at = `${p(h)}:${p(m)}:${p(s)}`

  if (dom === '*' && mon === '*' && dow === '*') return `每天 ${at}`
  if (dom === '*' && mon === '*') {
    const d = num(dow)
    if (d != null && d >= 0 && d <= 6) return `每${WEEKDAYS[d]} ${at}`
  }
  if (mon === '*' && dow === '*') {
    const d = num(dom)
    if (d != null) return `每月 ${d} 日 ${at}`
  }
  return ''
}

type Status = 'running' | 'ok' | 'fail' | 'idle'

function statusOf(job: Job): Status {
  if (job.running) return 'running'
  if (!job.last) return 'idle'
  return job.last.ok ? 'ok' : 'fail'
}

const STATUS_STYLE: Record<Status, { dot: string; text: string; label: string; chip: string }> = {
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
  idle: {
    dot: 'bg-muted-foreground/40',
    text: 'text-muted-foreground',
    label: '尚未执行',
    chip: 'bg-muted text-muted-foreground',
  },
}

function RunLine({ r }: { r: Run }) {
  return (
    <div className="flex items-start gap-2.5 py-2">
      <span
        className={cn(
          'mt-1 h-1.5 w-1.5 shrink-0 rounded-full',
          r.ok ? 'bg-emerald-500' : 'bg-rose-500',
        )}
      />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[12px]">
          <span className="font-mono text-foreground/80">{fmtTime(r.start)}</span>
          <span className="rounded bg-muted px-1.5 py-0.5 text-[10.5px] font-medium text-muted-foreground">
            {triggerLabel(r.trigger)}
          </span>
          <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
            <Timer className="h-3 w-3" />
            {fmtDuration(r.start, r.end)}
          </span>
        </div>
        {!r.ok && r.err && (
          <div className="mt-1 break-words rounded-md bg-rose-500/5 px-2 py-1 font-mono text-[11.5px] text-rose-600 dark:text-rose-400">
            {r.err}
          </div>
        )}
      </div>
    </div>
  )
}

function JobCard({
  job,
  accent,
  onRun,
}: {
  job: Job
  accent: ReturnType<typeof getColorSet>
  onRun: (name: string) => Promise<void>
}) {
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

  const meta = JOB_META[job.name] ?? { icon: Clock, label: '' }
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
                {job.last.ok ? (
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

        {job.last && !job.last.ok && job.last.err && (
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
              {job.history.map((r, i) => (
                <RunLine key={i} r={r} />
              ))}
            </div>
          )}
        </div>
      )}
    </Card>
  )
}

export default function SchedulerPage() {
  const cs = getColorSet('blue')
  const [jobs, setJobs] = useState<Job[]>([])
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/scheduler/jobs')
      setJobs(data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载任务失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const runJob = useCallback(
    async (name: string) => {
      try {
        await api.post(`/scheduler/jobs/${encodeURIComponent(name)}/run`)
        toast.success('已触发，稍后刷新查看结果')
        setTimeout(load, 1200)
      } catch (e: any) {
        toast.error(e?.response?.data?.error || e?.message || '触发失败')
      }
    },
    [load],
  )

  const okCount = jobs.filter((j) => statusOf(j) === 'ok').length
  const failCount = jobs.filter((j) => statusOf(j) === 'fail').length

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-4 flex items-end justify-between gap-4 sm:mb-6">
        <div className="hidden sm:block">
          <div className="flex items-center gap-3">
            <span className={cn('h-2.5 w-2.5 rounded-full', cs.dot)} />
            <h1 className="text-[26px] font-bold leading-none tracking-tight sm:text-[28px]">
              任务调度
            </h1>
          </div>
          <p className="mt-2 text-[12.5px] text-muted-foreground">
            进程内 cron 任务，支持手动触发与查看最近执行历史
          </p>
          {jobs.length > 0 && (
            <div className="mt-3 flex items-center gap-4 text-[12px] text-muted-foreground">
              <span className="flex items-center gap-1.5">
                <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/50" />
                共 {jobs.length} 个
              </span>
              {okCount > 0 && (
                <span className="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400">
                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                  {okCount} 成功
                </span>
              )}
              {failCount > 0 && (
                <span className="flex items-center gap-1.5 text-rose-600 dark:text-rose-400">
                  <span className="h-1.5 w-1.5 rounded-full bg-rose-500" />
                  {failCount} 失败
                </span>
              )}
            </div>
          )}
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={load}
          disabled={loading}
          className="w-full sm:w-auto"
        >
          {loading ? (
            <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
          ) : (
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
          )}
          刷新
        </Button>
      </div>

      {jobs.length === 0 ? (
        <Card className="flex flex-col items-center gap-2 py-16 text-center">
          <Clock className="h-8 w-8 text-muted-foreground/40" />
          <p className="text-[13px] text-muted-foreground">
            {loading ? '加载中…' : '没有已注册的任务'}
          </p>
        </Card>
      ) : (
        <div className="space-y-3">
          {jobs.map((j) => (
            <JobCard key={j.name} job={j} accent={cs} onRun={runJob} />
          ))}
        </div>
      )}
    </div>
  )
}
