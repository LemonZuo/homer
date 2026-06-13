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
import type { Channel } from './types'

interface ChannelDeleteDialogProps {
  target: Channel | null
  onClose: () => void
  onConfirm: () => void
}

export function ChannelDeleteDialog({
  target,
  onClose,
  onConfirm,
}: ChannelDeleteDialogProps) {
  return (
    <AlertDialog open={!!target} onOpenChange={(open) => !open && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除通道</AlertDialogTitle>
          <AlertDialogDescription>
            确认删除「{target?.name}」？仍被模块绑定时无法删除。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm}>删除</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
