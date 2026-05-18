import { useEffect, useMemo, useRef, useState } from 'react'
import { Copy } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '../ui/button'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '../ui/drawer'
import { STATUS_LABEL } from './utils'

export function LogDrawer({
  taskID,
  onClose,
}: {
  taskID: number | null
  onClose: () => void
}) {
  const [lines, setLines] = useState<string[]>([])
  const [done, setDone] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const open = taskID !== null

  useEffect(() => {
    if (taskID === null) return
    setLines([])
    setDone(null)
    const es = new EventSource(`/api/acme/tasks/${taskID}/stream`)
    es.addEventListener('log', (ev: MessageEvent) => {
      setLines((prev) => [...prev, ev.data])
    })
    es.addEventListener('done', (ev: MessageEvent) => {
      setDone(ev.data)
      es.close()
    })
    es.onerror = () => {
      es.close()
    }
    return () => es.close()
  }, [taskID])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines, done])

  const title = useMemo(() => (taskID ? `任务 #${taskID} 日志` : '日志'), [taskID])
  const logText = useMemo(() => (lines.length === 0 ? '' : lines.join('\n')), [lines])

  const copyLog = async () => {
    if (!logText) return
    try {
      await navigator.clipboard.writeText(logText)
      toast.success('日志已复制')
    } catch (e: any) {
      toast.error(e?.message || '复制失败')
    }
  }

  return (
    <Drawer
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose()
      }}
    >
      <DrawerContent>
        <DrawerHeader className="flex flex-row items-start justify-between gap-3">
          <div className="min-w-0 space-y-1">
            <DrawerTitle>{title}</DrawerTitle>
            <DrawerDescription>
              {done
                ? `状态：${STATUS_LABEL[done] || done}`
                : '实时推送（SSE）—— 关闭后可在任务历史里重看完整日志'}
            </DrawerDescription>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={copyLog}
            disabled={!logText}
            className="shrink-0"
            data-vaul-no-drag
          >
            <Copy className="h-3.5 w-3.5" />
            复制日志
          </Button>
        </DrawerHeader>
        <div className="flex-1 overflow-auto px-4 pb-4" data-vaul-no-drag>
          <pre className="cursor-text select-text whitespace-pre-wrap break-words rounded-lg border border-border bg-muted/40 p-3 font-mono text-[11.5px] leading-relaxed">
            {logText || '（暂无日志）'}
          </pre>
          <div ref={bottomRef} />
        </div>
      </DrawerContent>
    </Drawer>
  )
}
