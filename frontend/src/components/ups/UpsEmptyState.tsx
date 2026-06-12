import { Plug, Settings2 } from 'lucide-react'

import { Button } from '../ui/button'
import { Card } from '../ui/card'

export function UpsEmptyState({ onOpenHosts }: { onOpenHosts: () => void }) {
  return (
    <Card className="space-y-3 px-6 py-10 text-center">
      <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-muted">
        <Plug className="h-5 w-5 text-muted-foreground" />
      </div>
      <div className="text-[14px] font-medium">还没有 UPS 机器</div>
      <p className="mx-auto max-w-md text-[12.5px] text-muted-foreground">
        点右上「UPS 机器」新增要采样的目标。机器需先在远端装好 NUT(
        <code className="rounded bg-muted px-1">nut-client</code>) +
        <code className="ml-1 rounded bg-muted px-1">upsc</code>。
      </p>
      <Button variant="outline" size="sm" onClick={onOpenHosts}>
        <Settings2 className="mr-1.5 h-3.5 w-3.5" />
        打开 UPS 机器
      </Button>
    </Card>
  )
}
