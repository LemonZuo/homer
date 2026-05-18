import { useCallback, useState } from 'react'
import { Edit3, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../api'
import { cn } from '../../lib/utils'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '../ui/drawer'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../ui/alert-dialog'
import type { Forwarder } from './types'
import { ForwarderEditDialog } from './ForwarderEditDialog'

export function ForwarderManageDrawer({
  open,
  onOpenChange,
  forwarders,
  onReload,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  forwarders: Forwarder[]
  onReload: () => void
}) {
  const [editTarget, setEditTarget] = useState<Forwarder | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [delTarget, setDelTarget] = useState<Forwarder | null>(null)

  const doDelete = useCallback(async () => {
    if (!delTarget) return
    try {
      await api.delete(`/sms/forwarders/${delTarget.id}`)
      toast.success('已删除')
      setDelTarget(null)
      onReload()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }, [delTarget, onReload])

  return (
    <>
      <Drawer open={open} onOpenChange={onOpenChange}>
        <DrawerContent>
          <DrawerHeader>
            <DrawerTitle>转发器管理</DrawerTitle>
            <DrawerDescription>配置多台 SmsForwarder 服务端，页面顶部可切换</DrawerDescription>
          </DrawerHeader>
          <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
            <div className="flex justify-end [&>button]:h-10 [&>button]:w-full sm:[&>button]:h-8 sm:[&>button]:w-auto">
              <Button
                size="sm"
                onClick={() => {
                  setEditTarget(null)
                  setEditOpen(true)
                }}
              >
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                添加转发器
              </Button>
            </div>
            {forwarders.length === 0 ? (
              <p className="py-8 text-center text-[12.5px] text-muted-foreground">还没有转发器</p>
            ) : (
              forwarders.map((f) => (
                <Card key={f.id} className="px-4 py-3">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-mono text-[13px] font-medium">{f.name}</span>
                        <span
                          className={cn(
                            'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                            f.enabled
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                              : 'bg-muted text-muted-foreground',
                          )}
                        >
                          {f.enabled ? '启用' : '停用'}
                        </span>
                      </div>
                      <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                        {f.server_url}
                      </div>
                    </div>
                    <div className="flex gap-2 sm:contents">
                      <Button
                        size="sm"
                        variant="outline"
                        className="flex-1 sm:flex-none"
                        onClick={() => {
                          setEditTarget(f)
                          setEditOpen(true)
                        }}
                      >
                        <Edit3 className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="flex-1 hover:text-destructive sm:flex-none"
                        onClick={() => setDelTarget(f)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                </Card>
              ))
            )}
          </div>
        </DrawerContent>
      </Drawer>

      <ForwarderEditDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        target={editTarget}
        onSaved={onReload}
      />

      <AlertDialog open={!!delTarget} onOpenChange={(o) => !o && setDelTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除转发器</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除「{delTarget?.name}」？此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={doDelete}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
