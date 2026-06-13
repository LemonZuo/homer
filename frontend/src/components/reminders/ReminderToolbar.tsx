import { Plus, Search } from 'lucide-react'
import { cn } from '../../lib/utils'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Input } from '../ui/input'

interface ReminderToolbarProps {
  title: string
  count: number
  keyword: string
  onKeywordChange: (value: string) => void
  onAdd: () => void
  dotClassName: string
}

export function ReminderToolbar({
  title,
  count,
  keyword,
  onKeywordChange,
  onAdd,
  dotClassName,
}: ReminderToolbarProps) {
  return (
    <>
      <div className="mb-8 hidden flex-wrap items-end justify-between gap-3 sm:flex">
        <div className="flex items-center gap-3">
          <span className={cn('h-2 w-2 rounded-full', dotClassName)} />
          <h1 className="text-[28px] font-bold leading-none tracking-tight">{title}</h1>
          <Badge variant="muted" className="font-mono tabular-nums">
            {count}
          </Badge>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search
              size={14}
              className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              value={keyword}
              onChange={(e) => onKeywordChange(e.target.value)}
              placeholder="搜索"
              className="h-8 w-44 pl-7 text-[13px] transition-[width] focus-visible:w-60"
            />
          </div>
          <Button size="sm" onClick={onAdd}>
            <Plus className="h-3.5 w-3.5" />
            新增
          </Button>
        </div>
      </div>

      <div className="mb-4 sm:hidden">
        <div className="relative">
          <Search
            size={14}
            className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            value={keyword}
            onChange={(e) => onKeywordChange(e.target.value)}
            placeholder={`搜索 ${count} 条记录`}
            className="h-10 pl-8"
          />
        </div>
      </div>
    </>
  )
}
