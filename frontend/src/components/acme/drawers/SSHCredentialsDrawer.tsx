import { Edit3, KeyRound, Plus, Trash2 } from 'lucide-react'
import { Card } from '../../ui/card'
import { Button } from '../../ui/button'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '../../ui/drawer'
import type { SSHCredential } from '../types'
import { authLabel } from '../utils'

export function SSHCredentialsDrawer({
  open,
  onOpenChange,
  credentials,
  onAdd,
  onEdit,
  onDelete,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  credentials: SSHCredential[]
  onAdd: () => void
  onEdit: (c: SSHCredential) => void
  onDelete: (c: SSHCredential) => void
}) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>SSH 登录凭证</DrawerTitle>
          <DrawerDescription>
            一份登录身份可被多台机器共用；修改凭证后所有引用它的机器同步生效
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end [&>button]:h-10 [&>button]:w-full sm:[&>button]:h-8 sm:[&>button]:w-auto">
            <Button size="sm" onClick={onAdd}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              创建登录凭证
            </Button>
          </div>
          {credentials.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有登录凭证，点击「创建登录凭证」开始
            </p>
          ) : (
            credentials.map((c) => (
              <Card key={c.id} className="px-4 py-3">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                  <KeyRound className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-mono text-[13px] font-medium">{c.name}</div>
                    <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                      {c.username} · {authLabel(c.auth_type)}
                    </div>
                  </div>
                  <div className="flex gap-2 sm:contents">
                    <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onEdit(c)}>
                      <Edit3 className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="flex-1 hover:text-destructive sm:flex-none"
                      onClick={() => onDelete(c)}
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
  )
}
