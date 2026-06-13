import { Inbox, Loader2 } from 'lucide-react'

interface ReminderEmptyStateProps {
  loading: boolean
  hasKeyword: boolean
}

export function ReminderEmptyState({ loading, hasKeyword }: ReminderEmptyStateProps) {
  if (loading) {
    return (
      <div className="flex justify-center py-20 text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" />
      </div>
    )
  }

  return (
    <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed border-border py-16 text-center text-muted-foreground">
      <Inbox className="h-5 w-5 opacity-50" />
      <p className="text-[13px]">{hasKeyword ? '没有匹配的记录' : '暂无数据'}</p>
    </div>
  )
}
