import { Edit3, Plus, Trash2 } from 'lucide-react'
import { Card } from '../../ui/card'
import { Button } from '../../ui/button'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '../../ui/drawer'
import { cn } from '../../../lib/utils'
import type { AcmeAccount } from '../types'
import { caLabel } from '../utils'

export function AccountsDrawer({
  open,
  onOpenChange,
  accounts,
  onAdd,
  onEdit,
  onDelete,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  accounts: AcmeAccount[]
  onAdd: () => void
  onEdit: (a: AcmeAccount) => void
  onDelete: (a: AcmeAccount) => void
}) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>CA 账号</DrawerTitle>
          <DrawerDescription>
            维护 ACME CA、邮箱与 ZeroSSL EAB；域名可选择不同账号签发
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end [&>button]:h-10 [&>button]:w-full sm:[&>button]:h-8 sm:[&>button]:w-auto">
            <Button size="sm" onClick={onAdd}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加账号
            </Button>
          </div>
          {accounts.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有 CA 账号，点击「添加账号」开始
            </p>
          ) : (
            accounts.map((a) => (
              <Card key={a.id} className="px-4 py-3">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-mono text-[13px] font-medium">
                        {a.name}
                      </span>
                      <span
                        className={cn(
                          'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                          a.enabled
                            ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                            : 'bg-muted text-muted-foreground',
                        )}
                      >
                        {a.enabled ? '启用' : '停用'}
                      </span>
                    </div>
                    <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                      {caLabel(a.ca)} · {a.email}
                    </div>
                    {a.ca === 'custom' && (
                      <div
                        className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground"
                        title={a.directory_url}
                      >
                        {a.directory_url}
                      </div>
                    )}
                  </div>
                  <div className="flex gap-2 sm:contents">
                    <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onEdit(a)}>
                      <Edit3 className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="flex-1 hover:text-destructive sm:flex-none"
                      onClick={() => onDelete(a)}
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
