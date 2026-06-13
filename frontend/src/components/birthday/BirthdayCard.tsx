import { Bell, Loader2, Pencil, Trash2 } from 'lucide-react'
import { avatarColor, getColorSet } from '../../colors'
import { cn } from '../../lib/utils'
import { ReminderFieldRow } from '../reminders/ReminderFieldRow'
import { Button } from '../ui/button'
import { Card } from '../ui/card'
import { Switch } from '../ui/switch'
import { Tooltip, TooltipContent, TooltipTrigger } from '../ui/tooltip'
import type { Birthday } from './types'

interface BirthdayCardProps {
  record: Birthday
  accent: ReturnType<typeof getColorSet>
  notifying: boolean
  onNotify: (record: Birthday) => void
  onEdit: (record: Birthday) => void
  onDelete: (record: Birthday) => void
  onToggle: (record: Birthday, enabled: boolean) => void
}

export function BirthdayCard({
  record,
  accent,
  notifying,
  onNotify,
  onEdit,
  onDelete,
  onToggle,
}: BirthdayCardProps) {
  return (
    <Card
      className={cn(
        'group relative flex h-full flex-col overflow-hidden transition-[transform,box-shadow,border-color] duration-700 ease-[cubic-bezier(0.16,1,0.3,1)] will-change-transform hover:-translate-y-1',
        accent.border,
        accent.halo,
      )}
    >
      <div className="flex items-center gap-3 px-4 pt-4">
        <div
          className={cn(
            'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-[13px] font-medium text-white shadow-sm',
            avatarColor(record.name || '?'),
          )}
        >
          {(record.name || '?').charAt(0).toUpperCase()}
        </div>
        <div className="min-w-0 flex-1">
          <div className="truncate text-[14px] font-semibold tracking-tight">
            {record.name || '(无姓名)'}
          </div>
          <div className="mt-0.5 truncate text-[12px] text-muted-foreground">
            {[record.birthday, record.zodiac].filter(Boolean).join(' · ')}
          </div>
        </div>
        <div className="flex shrink-0 gap-0.5 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7"
                onClick={() => onNotify(record)}
                disabled={notifying}
                aria-label="发送提醒"
              >
                {notifying ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Bell className="h-3.5 w-3.5" />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent>发送提醒</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7"
                onClick={() => onEdit(record)}
                aria-label="编辑"
              >
                <Pencil className="h-3.5 w-3.5" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>编辑</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 hover:text-destructive"
                onClick={() => onDelete(record)}
                aria-label="删除"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>删除</TooltipContent>
          </Tooltip>
        </div>
      </div>

      <div className="mt-3 space-y-0 px-4 pb-3">
        <ReminderFieldRow label="公历生日" value={record.birthday} />
        <ReminderFieldRow label="农历生日" value={record.chinese_birthday} />
        <ReminderFieldRow label="生肖" value={record.zodiac} />
        <div className="flex items-center gap-3 py-1 text-[12.5px]">
          <span className="w-16 shrink-0 text-muted-foreground">启用提醒</span>
          <div className="flex min-w-0 flex-1 items-center justify-end">
            <Switch checked={record.enabled} onChange={(v) => onToggle(record, v)} size="sm" />
          </div>
        </div>
      </div>
    </Card>
  )
}
