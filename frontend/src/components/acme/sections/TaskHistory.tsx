import { Loader2, RefreshCw, ScrollText } from 'lucide-react'
import { Card } from '../../ui/card'
import { Button } from '../../ui/button'
import { Select } from '../../ui/select'
import { cn } from '../../../lib/utils'
import { TaskPager } from '../TaskPager'
import { KIND_LABEL, STATUS_LABEL, STATUS_STYLE, TASK_PAGE_SIZES, fmtDateTime } from '../utils'
import type { Task } from '../types'

export function TaskHistory({
  tasks,
  loading,
  taskStatus,
  onStatusChange,
  taskPage,
  taskPageSize,
  taskTotal,
  onGo,
  onPageSizeChange,
  onShowLog,
  onRetry,
  busy,
}: {
  tasks: Task[]
  loading: boolean
  taskStatus: string
  onStatusChange: (status: string) => void
  taskPage: number
  taskPageSize: number
  taskTotal: number
  onGo: (page: number) => void
  onPageSizeChange: (size: number) => void
  onShowLog: (id: number) => void
  onRetry: (id: number) => void
  busy: string | null
}) {
  return (
    <>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <h2 className="text-[14px] font-semibold tracking-tight">任务历史</h2>
          <Select<string>
            className="h-8 w-28 text-[12px]"
            value={taskStatus}
            onChange={onStatusChange}
            options={[
              { value: '', label: '全部状态' },
              ...(['pending', 'running', 'success', 'failed', 'retrying'] as const).map(
                (s) => ({ value: s, label: STATUS_LABEL[s] || s }),
              ),
            ]}
          />
        </div>
        <div className="w-full sm:w-auto">
          <TaskPager
            page={taskPage}
            pageSize={taskPageSize}
            total={taskTotal}
            onGo={onGo}
            onPageSizeChange={onPageSizeChange}
          />
        </div>
      </div>
      <div className="space-y-2">
        {tasks.map((t) => (
          <Card
            key={t.id}
            className="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3 text-[12.5px]"
          >
            <span
              className={cn(
                'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                STATUS_STYLE[t.status] || 'bg-muted text-muted-foreground',
              )}
            >
              {STATUS_LABEL[t.status] || t.status}
              {(t.max_attempt ?? 1) > 1 && (t.attempt ?? 0) > 0 && (
                <span className="ml-1 opacity-70">
                  {t.attempt}/{t.max_attempt}
                </span>
              )}
            </span>
            <span className="font-mono">#{t.id}</span>
            <span className="font-medium">{t.main_domain}</span>
            <span className="text-muted-foreground">{KIND_LABEL[t.kind] || t.kind}</span>
            <span className="w-full font-mono text-[11.5px] text-muted-foreground sm:ml-auto sm:w-auto">
              {fmtDateTime(t.started_at)}
            </span>
            <Button
              size="sm"
              variant="outline"
              className="h-9 w-full sm:h-8 sm:w-auto"
              onClick={() => onShowLog(t.id)}
            >
              <ScrollText className="mr-1.5 h-3.5 w-3.5" />
              日志
            </Button>
            {(t.config_id ?? 0) > 0 &&
              (t.status === 'failed' || t.status === 'retrying') && (
                <Button
                  size="sm"
                  variant="outline"
                  className="h-9 w-full sm:h-8 sm:w-auto"
                  disabled={busy === `retry-${t.id}`}
                  onClick={() => onRetry(t.id)}
                >
                  {busy === `retry-${t.id}` ? (
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  重试
                </Button>
              )}
          </Card>
        ))}
        {!loading && tasks.length === 0 && (
          <p className="py-6 text-center text-[12.5px] text-muted-foreground">
            还没有任务
          </p>
        )}
      </div>

      {taskTotal > TASK_PAGE_SIZES[0] && (
        <div className="mt-3 hidden justify-end sm:flex">
          <TaskPager
            page={taskPage}
            pageSize={taskPageSize}
            total={taskTotal}
            onGo={onGo}
            onPageSizeChange={onPageSizeChange}
          />
        </div>
      )}
    </>
  )
}
