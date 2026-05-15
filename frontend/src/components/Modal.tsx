import { useEffect, useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from './ui/drawer'

interface ModalProps {
  open: boolean
  title: string
  description?: string
  onClose: () => void
  children: React.ReactNode
}

function useIsDesktop() {
  const [is, setIs] = useState(() =>
    typeof window === 'undefined' ? true : window.matchMedia('(min-width: 640px)').matches,
  )
  useEffect(() => {
    const m = window.matchMedia('(min-width: 640px)')
    const fn = () => setIs(m.matches)
    m.addEventListener('change', fn)
    return () => m.removeEventListener('change', fn)
  }, [])
  return is
}

/**
 * 响应式对话框：桌面端 Radix Dialog，移动端 vaul Drawer。
 */
export default function Modal({ open, title, description, onClose, children }: ModalProps) {
  const isDesktop = useIsDesktop()

  if (isDesktop) {
    return (
      <Dialog open={open} onOpenChange={(o) => { if (!o) onClose() }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            {description && (
              <p className="text-[12.5px] text-muted-foreground">{description}</p>
            )}
          </DialogHeader>
          <div className="mt-1">{children}</div>
        </DialogContent>
      </Dialog>
    )
  }

  return (
    <Drawer open={open} onOpenChange={(o) => { if (!o) onClose() }}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>{title}</DrawerTitle>
          {description && (
            <p className="text-[12.5px] text-muted-foreground">{description}</p>
          )}
        </DrawerHeader>
        <div className="overflow-y-auto px-4 pb-6">{children}</div>
      </DrawerContent>
    </Drawer>
  )
}
