import { Clock, Loader2, RefreshCw } from 'lucide-react'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { cn } from '../lib/utils'
import { JobCard } from './scheduler/JobCard'
import { statusOf } from './scheduler/status'
import { useSchedulerJobs } from './scheduler/useSchedulerJobs'

export default function Scheduler() {
  const cs = getColorSet('blue')
  const { jobs, loading, load, runJob } = useSchedulerJobs()

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
