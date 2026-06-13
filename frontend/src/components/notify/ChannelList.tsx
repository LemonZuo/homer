import { Edit3, Loader2, Send, Trash2 } from 'lucide-react'
import { cn } from '../../lib/utils'
import { Button } from '../ui/button'
import { Card } from '../ui/card'
import type { Channel, TypeMeta } from './types'
import { typeLabel } from './utils'

interface ChannelListProps {
  channels: Channel[]
  types: TypeMeta[]
  testingID: number | null
  onTest: (id: number) => void
  onEdit: (channel: Channel) => void
  onDelete: (channel: Channel) => void
}

export function ChannelList({
  channels,
  types,
  testingID,
  onTest,
  onEdit,
  onDelete,
}: ChannelListProps) {
  return (
    <Card className="mb-4 px-4 py-4">
      <div className="mb-3 text-[13px] font-medium">通道</div>
      {channels.length === 0 ? (
        <p className="rounded-md border border-dashed border-border py-8 text-center text-[12px] text-muted-foreground">
          还没有通道，点击右上角「新增通道」
        </p>
      ) : (
        <div className="space-y-2">
          {channels.map((ch) => (
            <Card key={ch.id} className="px-4 py-3">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="truncate font-mono text-[13px] font-medium">{ch.name}</span>
                    <span className="rounded-md bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                      {typeLabel(types, ch.type)}
                    </span>
                    <span
                      className={cn(
                        'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                        ch.enabled
                          ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                          : 'bg-muted text-muted-foreground',
                      )}
                    >
                      {ch.enabled ? '启用' : '停用'}
                    </span>
                    {ch.ref_count > 0 && (
                      <span className="shrink-0 rounded-md bg-blue-500/10 px-1.5 py-0.5 text-[11px] font-medium text-blue-600 dark:text-blue-400">
                        {ch.ref_count}
                      </span>
                    )}
                  </div>
                </div>
                <div className="flex gap-2 sm:contents">
                  <Button
                    size="sm"
                    variant="outline"
                    className="flex-1 sm:flex-none"
                    disabled={testingID === ch.id}
                    onClick={() => onTest(ch.id)}
                  >
                    {testingID === ch.id ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Send className="h-3.5 w-3.5" />
                    )}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="flex-1 sm:flex-none"
                    onClick={() => onEdit(ch)}
                  >
                    <Edit3 className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="flex-1 hover:text-destructive disabled:hover:text-current sm:flex-none"
                    disabled={ch.ref_count > 0}
                    title={ch.ref_count > 0 ? '仍被模块绑定，请先解绑' : undefined}
                    onClick={() => onDelete(ch)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
    </Card>
  )
}
