import { useEffect, useMemo, useRef, useState } from 'react'
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

  return (
    <Drawer
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose()
      }}
    >
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>{title}</DrawerTitle>
          <DrawerDescription>
            {done
              ? `状态：${STATUS_LABEL[done] || done}`
              : '实时推送（SSE）—— 关闭后可在任务历史里重看完整日志'}
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 overflow-auto px-4 pb-4">
          <pre className="whitespace-pre-wrap break-all rounded-lg border border-border bg-muted/40 p-3 font-mono text-[11.5px] leading-relaxed">
            {lines.length === 0 ? '（暂无日志）' : lines.join('\n')}
            <div ref={bottomRef} />
          </pre>
        </div>
      </DrawerContent>
    </Drawer>
  )
}
