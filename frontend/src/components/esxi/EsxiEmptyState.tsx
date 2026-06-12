import { Server, Settings2 } from 'lucide-react'

import { Button } from '../ui/button'
import { Card } from '../ui/card'

export function EsxiEmptyState({ onOpenHosts }: { onOpenHosts: () => void }) {
  return (
    <Card className="space-y-3 px-6 py-10 text-center">
      <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-muted">
        <Server className="h-5 w-5 text-muted-foreground" />
      </div>
      <div className="text-[14px] font-medium">还没有 ESXi 机器</div>
      <p className="mx-auto max-w-md text-[12.5px] text-muted-foreground">
        点右上「ESXi 机器」新增要采样的主机;需先开放 ESXi 的 SSH(默认是关闭的)。
      </p>
      <Button variant="outline" size="sm" onClick={onOpenHosts}>
        <Settings2 className="mr-1.5 h-3.5 w-3.5" />
        打开 ESXi 机器
      </Button>
    </Card>
  )
}
