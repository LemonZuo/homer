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
              <Card key={c.id} className="flex items-center gap-3 px-3 py-2.5">
                <KeyRound className="h-4 w-4 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="truncate font-mono text-[13px] font-medium">{c.name}</span>
                    <span
                      className={
                        c.ref_count
                          ? 'shrink-0 rounded-md bg-emerald-500/10 px-1.5 py-0.5 text-[11px] font-medium text-emerald-600 dark:text-emerald-400'
                          : 'shrink-0 rounded-md bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground'
                      }
                    >
                      {c.ref_count ? `${c.ref_count} 台机器` : '未使用'}
                    </span>
                  </div>
                  <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                    {c.username} · {authLabel(c.auth_type)}
                  </div>
                </div>
                <div className="flex shrink-0 gap-2">
                  <Button
                    size="icon"
                    variant="outline"
                    className="h-9 w-9"
                    onClick={() => onEdit(c)}
                  >
                    <Edit3 className="h-4 w-4" />
                  </Button>
                  <Button
                    size="icon"
                    variant="outline"
                    className="h-9 w-9 hover:text-destructive"
                    onClick={() => onDelete(c)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </Card>
            ))
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}
