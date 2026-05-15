import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '../ui/button'
import { TASK_PAGE_SIZES } from './utils'

export function TaskPager({
  page,
  pageSize,
  total,
  onGo,
  onPageSizeChange,
}: {
  page: number
  pageSize: number
  total: number
  onGo: (page: number) => void
  onPageSizeChange: (size: number) => void
}) {
  if (total <= TASK_PAGE_SIZES[0]) return null
  const pages = Math.ceil(total / pageSize)
  return (
    <div className="flex flex-wrap items-center gap-2 text-[12px] text-muted-foreground sm:gap-3">
      <span className="font-mono">
        {page} / {pages}（共 {total} 条）
      </span>
      <select
        className="ml-auto h-8 rounded-md border border-input bg-background px-2 text-[12px] sm:ml-0"
        value={pageSize}
        onChange={(e) => onPageSizeChange(Number(e.target.value))}
      >
        {TASK_PAGE_SIZES.map((s) => (
          <option key={s} value={s}>
            {s} 条/页
          </option>
        ))}
      </select>
      <div className="flex w-full gap-2 sm:contents">
        <Button
          size="sm"
          variant="outline"
          className="h-9 flex-1 sm:h-8 sm:flex-none"
          disabled={page <= 1}
          onClick={() => onGo(page - 1)}
        >
          <ChevronLeft className="mr-1 h-3.5 w-3.5" />
          上一页
        </Button>
        <Button
          size="sm"
          variant="outline"
          className="h-9 flex-1 sm:h-8 sm:flex-none"
          disabled={page >= pages}
          onClick={() => onGo(page + 1)}
        >
          下一页
          <ChevronRight className="ml-1 h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  )
}
