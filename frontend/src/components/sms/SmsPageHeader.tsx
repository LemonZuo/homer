import { Settings2 } from 'lucide-react'
import { getColorSet } from '../../colors'
import { cn } from '../../lib/utils'
import { Button } from '../ui/button'
import { Select } from '../ui/select'
import type { Forwarder } from './types'

interface SmsPageHeaderProps {
  accent: ReturnType<typeof getColorSet>
  selectedID: number | null
  enabledList: Forwarder[]
  onSelect: (id: number) => void
  onManage: () => void
}

export function SmsPageHeader({
  accent,
  selectedID,
  enabledList,
  onSelect,
  onManage,
}: SmsPageHeaderProps) {
  return (
    <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div className="hidden sm:block">
        <div className="flex items-center gap-3">
          <span className={cn('h-2 w-2 rounded-full', accent.dot)} />
          <h1 className="text-[28px] font-bold leading-none tracking-tight">短信转发器</h1>
        </div>
        <p className="mt-2 text-[12.5px] text-muted-foreground">
          通过 SmsForwarder Android 服务收发短信；签名鉴权模式
        </p>
      </div>
      <div className="flex items-center gap-2">
        <Select<number>
          className="h-9 min-w-[140px] flex-1 sm:flex-none"
          value={selectedID ?? 0}
          onChange={onSelect}
          placeholder="无可用转发器"
          options={enabledList.map((f) => ({ value: f.id, label: f.name }))}
        />
        <Button variant="outline" size="sm" onClick={onManage}>
          <Settings2 className="mr-1.5 h-3.5 w-3.5" />
          管理
        </Button>
      </div>
    </div>
  )
}
